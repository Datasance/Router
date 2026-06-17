package qdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse-iofog/router/internal/config"
	types "github.com/eclipse-iofog/router/internal/resources/skuppertypes"
	utils "github.com/eclipse-iofog/router/internal/routerutil"
	amqp "github.com/interconnectedcloud/go-amqp"
)

type RouterNode struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NextHop string `json:"nextHop"`
	Address string `json:"address"`
}

func (r *RouterNode) IsSelf() bool {
	return r.NextHop == "(self)"
}

type Connection struct {
	Container  string `json:"container"`
	OperStatus string `json:"operStatus"`
	Host       string `json:"host"`
	Role       string `json:"role"`
	Active     bool   `json:"active"`
	Dir        string `json:"dir"`
}

type Agent struct {
	connection *amqp.Client
	session    *amqp.Session
	sender     *amqp.Sender
	anonymous  *amqp.Sender
	receiver   *amqp.Receiver
	local      *Router
	closed     bool
}

type Router struct {
	ID          string
	Address     string
	Edge        bool
	Site        SiteMetadata
	Version     string
	ConnectedTo []string
}

type SiteMetadata struct {
	ID       string `json:"id,omitempty"`
	Version  string `json:"version,omitempty"`
	Platform string `json:"platform,omitempty"`
}

func GetSiteMetadata(metadata string) SiteMetadata {
	result := SiteMetadata{}
	err := json.Unmarshal([]byte(metadata), &result)
	if err != nil {
		log.Printf("Assuming old format for router metadata %s: %s", metadata, err)
		// assume old format, where metadata just holds site id
		result.ID = metadata
	}
	return result
}

func getSiteMetadataString(siteID string, version string) string {
	siteDetails := SiteMetadata{
		ID:       siteID,
		Version:  version,
		Platform: string(config.GetPlatform()),
	}
	metadata, _ := json.Marshal(siteDetails)
	return string(metadata)
}

type recordType interface {
	toRecord() Record
}

type Record map[string]any

func (r Record) AsString(field string) string {
	if value, ok := r[field].(string); ok {
		return value
	}
	return ""
}

func (r Record) AsBool(field string) bool {
	if value, ok := r[field].(bool); ok {
		return value
	}
	return false
}

func (r Record) AsInt(field string) int {
	value, _ := AsInt(r[field])
	return value
}

func (r Record) AsUint64(field string) uint64 {
	value, _ := AsUint64(r[field])
	return value
}

func (r Record) AsRecord(field string) Record {
	if value, ok := r[field].(map[string]any); ok {
		return value
	}
	return nil
}

func asTCPEndpoint(record Record) TCPEndpoint {
	endpoint := TCPEndpoint{
		Name:       record.AsString("name"),
		Host:       record.AsString("host"),
		Port:       record.AsString("port"),
		Address:    record.AsString("address"),
		SiteID:     record.AsString("siteID"),
		SslProfile: record.AsString("sslProfile"),
		ProcessID:  record.AsString("processId"),
	}
	if value, ok := record["verifyHostname"]; ok {
		if verify, ok := value.(bool); ok {
			endpoint.VerifyHostname = &verify
		}
	}
	return endpoint
}

func asConnection(record Record) Connection {
	return Connection{
		Role:       record.AsString("role"),
		Container:  record.AsString("container"),
		Host:       record.AsString("host"),
		OperStatus: record.AsString("operStatus"),
		Dir:        record.AsString("dir"),
		Active:     record.AsBool("active"),
	}
}

func asRouterNode(record Record) RouterNode {
	return RouterNode{
		ID:      record.AsString("id"),
		Name:    record.AsString("name"),
		Address: record.AsString("address"),
		NextHop: record.AsString("nextHop"),
	}
}

func asRouter(record Record) *Router {
	r := Router{
		ID:      record.AsString("id"),
		Site:    GetSiteMetadata(record.AsString("metadata")),
		Version: record.AsString("version"),
	}
	r.Edge = record.AsString("mode") == "edge"
	r.Address = GetRouterAgentAddress(r.ID, r.Edge)
	return &r
}

func (r *RouterNode) AsRouter() *Router {
	return &Router{
		ID: r.ID,
		// SiteID ???
		Address: r.Address,
		Edge:    false, // RouterNode is always an interior
	}
}

type AgentPool struct {
	url    string
	config TLSConfigRetriever
	pool   chan *Agent
}

func NewAgentPool(url string, config TLSConfigRetriever) *AgentPool {
	return &AgentPool{
		url:    url,
		config: config,
		pool:   make(chan *Agent, 10),
	}
}

func (p *AgentPool) Get() (*Agent, error) {
	var a *Agent
	var err error
	select {
	case a = <-p.pool:
	default:
		a, err = Connect(p.url, p.config)
	}
	return a, err
}

func (p *AgentPool) Put(a *Agent) {
	if !a.closed {
		select {
		case p.pool <- a:
		default:
			_ = a.Close()
		}
	}
}

func Connect(url string, config TLSConfigRetriever) (*Agent, error) {
	factory := ConnectionFactory{
		url:    url,
		config: config,
	}
	return newAgent(&factory)
}

func newAgent(factory *ConnectionFactory) (*Agent, error) {
	client, err := factory.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	connection, ok := client.(*AMQPConnection)
	if !ok {
		return nil, fmt.Errorf("unexpected connection type %T", client)
	}
	receiver, err := connection.session.NewReceiver(
		amqp.LinkSourceAddress(""),
		amqp.LinkAddressDynamic(),
		amqp.LinkCredit(10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create receiver: %w", err)
	}
	sender, err := connection.session.NewSender(
		amqp.LinkTargetAddress("$management"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sender: %w", err)
	}
	anonymous, err := connection.session.NewSender()
	if err != nil {
		return nil, fmt.Errorf("failed to create anonymous sender: %w", err)
	}
	a := &Agent{
		connection: connection.client,
		session:    connection.session,
		sender:     sender,
		anonymous:  anonymous,
		receiver:   receiver,
	}
	a.local, err = a.GetLocalRouter()
	if err != nil {
		return a, fmt.Errorf("failed to lookup local router details: %w", err)
	}
	return a, nil
}

func (a *Agent) newReceiver(address string) (*amqp.Receiver, error) {
	return a.session.NewReceiver(
		amqp.LinkSourceAddress(address),
		amqp.LinkCredit(10),
	)
}

func (a *Agent) Close() error {
	a.closed = true
	return a.connection.Close()
}

func isOk(code int) bool {
	return code >= 200 && code < 300
}

func cleanup(input any) any {
	switch input := input.(type) {
	case map[any]any:
		m := make(map[string]any)
		for k, v := range input {
			m[k.(string)] = cleanup(v)
		}
		return m
	case map[string]any:
		for k, v := range input {
			input[k] = cleanup(v)
		}
		return input
	default:
		return input
	}
}

func makeRecord(fields []string, values []any) Record {
	record := Record{}
	for i, name := range fields {
		record[name] = cleanup(values[i])
	}
	return record
}

func stringify(items []any) []string {
	s := make([]string, len(items))
	for i := range items {
		s[i] = fmt.Sprintf("%v", items[i])
	}
	return s
}

func GetRouterAgentAddress(id string, edge bool) string {
	if edge {
		return "amqp:/_edge/" + id + "/$management"
	}
	return "amqp:/_topo/0/" + id + "/$management"
}

func GetRouterAddress(id string, edge bool) string {
	if edge {
		return "amqp:/_edge/" + id
	}
	return "amqp:/_topo/0/" + id
}

func (a *Agent) request(operation string, typename string, name string, attributes map[string]any) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	var request amqp.Message
	var properties amqp.MessageProperties
	properties.ReplyTo = a.receiver.Address()
	properties.CorrelationID = uint64(1)
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]any)
	request.ApplicationProperties["operation"] = operation
	request.ApplicationProperties["type"] = typename
	request.ApplicationProperties["name"] = name
	if attributes != nil {
		request.Value = attributes
	}

	if err := a.sender.Send(ctx, &request); err != nil {
		_ = a.Close()
		return fmt.Errorf("could not send request: %w", err)
	}

	response, err := a.receiver.Receive(ctx)
	if err != nil {
		_ = a.Close()
		return fmt.Errorf("failed to receive response: %w", err)
	}
	_ = response.Accept()
	if status, ok := AsInt(response.ApplicationProperties["statusCode"]); !ok && !isOk(status) {
		return fmt.Errorf("query failed with: %s", response.ApplicationProperties["statusDescription"])
	}
	return nil
}

func (a *Agent) Create(typename string, name string, entity recordType) error {
	attributes := entity.toRecord()
	log.Println("CREATE", typename, name, attributes)
	return a.request("CREATE", typename, name, attributes)
}

func (a *Agent) Update(typename string, name string, entity recordType) error {
	attributes := entity.toRecord()
	log.Println("UPDATE", typename, name, attributes)
	return a.request("UPDATE", typename, name, attributes)
}

func (a *Agent) Delete(typename string, name string) error {
	if name == "" {
		return fmt.Errorf("cannot delete entity of type %s with no name", typename)
	}
	log.Println("DELETE", typename, name)
	return a.request("DELETE", typename, name, nil)
}

func (a *Agent) Query(typename string, attributes []string) ([]Record, error) {
	return a.QueryRouterNode(typename, attributes, nil)
}

func (a *Agent) QueryRouterNode(typename string, attributes []string, node *RouterNode) ([]Record, error) {
	var address string
	if node != nil {
		address = node.Address
	}
	return a.QueryByAgentAddress(typename, attributes, address)
}

func AsInt(value any) (int, bool) {
	switch value := value.(type) {
	case uint8:
		return int(value), true
	case uint16:
		return int(value), true
	case uint32:
		return int(value), true
	case uint64:
		return int(value), true
	case int8:
		return int(value), true
	case int16:
		return int(value), true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case int:
		return value, true
	default:
		return 0, false
	}
}

func AsUint64(value any) (uint64, bool) {
	switch value := value.(type) {
	case uint8:
		return uint64(value), true
	case uint16:
		return uint64(value), true
	case uint32:
		return uint64(value), true
	case uint64:
		return value, true
	case int8:
		return uint64(value), true
	case int16:
		return uint64(value), true
	case int32:
		return uint64(value), true
	case int64:
		return uint64(value), true
	case int:
		return uint64(value), true
	default:
		return 0, false
	}
}

func (a *Agent) QueryByAgentAddress(typename string, attributes []string, agent string) ([]Record, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	var request amqp.Message
	var properties amqp.MessageProperties
	properties.ReplyTo = a.receiver.Address()
	properties.CorrelationID = uint64(1)
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]any)
	request.ApplicationProperties["operation"] = "QUERY"
	request.ApplicationProperties["entityType"] = typename
	var body = make(map[string]any)
	body["attributeNames"] = attributes
	request.Value = body

	var err error
	if agent == "" {
		err = a.sender.Send(ctx, &request)
	} else {
		request.Properties.To = agent
		err = a.anonymous.Send(ctx, &request)
	}
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("could not send request: %w", err)
	}

	response, err := a.receiver.Receive(ctx)
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}
	_ = response.Accept()
	if status, ok := AsInt(response.ApplicationProperties["statusCode"]); ok && isOk(status) {
		if top, ok := response.Value.(map[string]any); ok {
			records := []Record{}
			attrNames, ok := top["attributeNames"].([]any)
			if !ok {
				return nil, fmt.Errorf("bad response attribute names: %v", top["attributeNames"])
			}
			fields := stringify(attrNames)
			results, ok := top["results"].([]any)
			if !ok {
				return nil, fmt.Errorf("bad response results: %v", top["results"])
			}
			for _, row := range results {
				rowValues, ok := row.([]any)
				if !ok {
					return nil, fmt.Errorf("bad response row: %v", row)
				}
				records = append(records, makeRecord(fields, rowValues))
			}
			return records, nil
		}
		return nil, fmt.Errorf("bad response: %s", response.Value)
	}
	return nil, fmt.Errorf("query failed with: %s", response.ApplicationProperties["statusDescription"])
}

type Query struct {
	typename   string
	attributes []string
	agent      string
}

func queryAllAgents(typename string, agents []string) []Query {
	queries := make([]Query, len(agents))
	for i, a := range agents {
		queries[i].typename = typename
		queries[i].attributes = []string{}
		queries[i].agent = a
	}
	return queries
}

func queryAllTypes(typenames []string, agent string) []Query {
	queries := make([]Query, len(typenames))
	for i, t := range typenames {
		queries[i].typename = t
		queries[i].attributes = []string{}
		queries[i].agent = agent
	}
	return queries
}

func queryAllAgentsForAllTypes(typenames []string, agents []string) []Query {
	queries := make([]Query, len(agents)*len(typenames))
	i := 0
	for _, t := range typenames {
		for _, a := range agents {
			queries[i].typename = t
			queries[i].attributes = []string{}
			queries[i].agent = a
			i++
		}
	}
	return queries
}

func (a *Agent) BatchQuery(queries []Query) ([][]Record, error) {
	_, _ = fmt.Printf("BatchQuery(%v)\n", queries)
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	batchResults := make([][]Record, len(queries))
	for i, q := range queries {
		var request amqp.Message
		var properties amqp.MessageProperties
		properties.ReplyTo = a.receiver.Address()
		properties.CorrelationID = uint64(i)
		request.Properties = &properties
		request.ApplicationProperties = make(map[string]any)
		request.ApplicationProperties["operation"] = "QUERY"
		request.ApplicationProperties["entityType"] = q.typename
		var body = make(map[string]any)
		body["attributeNames"] = q.attributes
		request.Value = body

		var err error
		if q.agent == "" {
			err = a.sender.Send(ctx, &request)
		} else {
			request.Properties.To = q.agent
			err = a.anonymous.Send(ctx, &request)
		}
		if err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("could not send request: %w", err)
		}
	}
	errors := []string{}
	for i := 0; i < len(queries); i++ {
		_, _ = fmt.Printf("Waiting for response %d of %d\n", (i + 1), len(queries))
		response, err := a.receiver.Receive(ctx)
		if err != nil {
			_ = a.Close()
			return nil, fmt.Errorf("failed to receive response: %w", err)
		}
		_ = response.Accept()
		responseIndex, ok := response.Properties.CorrelationID.(uint64)
		if !ok {
			errors = append(errors, fmt.Sprintf("Could not get correct correlation id from response: %#v (%T)", response.Properties.CorrelationID, response.Properties.CorrelationID))
		} else {
			if status, ok := AsInt(response.ApplicationProperties["statusCode"]); ok && isOk(status) {
				if top, ok := response.Value.(map[string]any); ok {
					records := []Record{}
					attrNames, ok := top["attributeNames"].([]any)
					if !ok {
						errors = append(errors, fmt.Sprintf("bad response attribute names: %v", top["attributeNames"]))
						continue
					}
					fields := stringify(attrNames)
					results, ok := top["results"].([]any)
					if !ok {
						errors = append(errors, fmt.Sprintf("bad response results: %v", top["results"]))
						continue
					}
					for _, row := range results {
						rowValues, ok := row.([]any)
						if !ok {
							errors = append(errors, fmt.Sprintf("bad response row: %v", row))
							continue
						}
						records = append(records, makeRecord(fields, rowValues))
					}
					batchResults[responseIndex] = records
				} else {
					errors = append(errors, fmt.Sprintf("bad response: %s", response.Value))
				}
			} else {
				errors = append(errors, fmt.Sprintf("query failed with: %s", response.ApplicationProperties["statusDescription"]))
			}
		}
	}
	if len(errors) > 0 {
		return nil, fmt.Errorf("%s", strings.Join(errors, ", "))
	}
	return batchResults, nil
}

func (a *Agent) GetInteriorNodes() ([]RouterNode, error) {
	var address string
	var err error
	if a.isEdgeRouter() {
		address, err = a.getInteriorAddressForUplink()
		if err != nil {
			return nil, fmt.Errorf("could not determine interior agent address for edge router: %w", err)
		}
	}
	records, err := a.QueryByAgentAddress("io.skupper.router.router.node", []string{}, address)
	if err != nil {
		return nil, err
	}
	_, _ = fmt.Printf("Interior nodes are %v\n", records)
	nodes := make([]RouterNode, len(records))
	for i, r := range records {
		nodes[i] = asRouterNode(r)
	}
	return nodes, nil
}

func (a *Agent) GetConnections() ([]Connection, error) {
	return a.GetConnectionsFor("")
}

func (a *Agent) GetConnectionsFor(agent string) ([]Connection, error) {
	records, err := a.Query("io.skupper.router.connection", []string{})
	if err != nil {
		return nil, err
	}
	connections := make([]Connection, len(records))
	for i, r := range records {
		connections[i] = asConnection(r)
	}
	return connections, nil
}

func getAddressesFor(routers []Router) []string {
	agents := make([]string, len(routers))
	for i, r := range routers {
		agents[i] = r.Address + "/$management"
	}
	return agents
}

func getBridgeServerAddressesFor(routers []Router) []string {
	agents := make([]string, len(routers))
	for i, r := range routers {
		agents[i] = r.ID + "/bridge-server/$management"
	}
	return agents
}

func GetRoutersForSite(routers []Router, siteID string) []Router {
	list := []Router{}
	for _, r := range routers {
		if r.Site.ID == siteID {
			list = append(list, r)
		}
	}
	return list
}

func (a *Agent) GetAllRouters() ([]Router, error) {
	nodes, err := a.GetInteriorNodes()
	if err != nil {
		return nil, err
	}
	routers := []Router{}
	routersFiltered := []Router{}
	for _, n := range nodes {
		routers = append(routers, *n.AsRouter())
	}
	edges, err := a.getAllEdgeRouters(getAddressesFor(routers))
	if err != nil {
		return nil, err
	}
	routers = append(routers, edges...)
	err = a.getSiteIDs(routers)
	if err != nil {
		return nil, err
	}
	isSvcRouter := func(edgeId string) bool {
		for _, r := range routers {
			if r.Edge {
				continue
			}
			// podman svc
			if strings.HasPrefix(edgeId, r.Site.ID+"-") || strings.HasSuffix(edgeId, "-"+r.Site.ID) {
				return true
			}
		}
		return false
	}
	for _, r := range routers {
		if !r.Edge || !isSvcRouter(r.Site.ID) {
			routersFiltered = append(routersFiltered, r)
		}
	}
	routers = routersFiltered
	err = a.getConnectedTo(routers)
	if err != nil {
		return nil, err
	}
	return routers, nil
}

func (a *Agent) getConnectionsForAll(agents []string) ([]Connection, error) {
	connections := []Connection{}
	results, err := a.BatchQuery(queryAllAgents("io.skupper.router.connection", agents))
	if err != nil {
		return nil, err
	}
	for _, records := range results {
		for _, r := range records {
			connections = append(connections, asConnection(r))
		}
	}
	return connections, nil
}

func (a *Agent) getSiteIDs(routers []Router) error {
	results, err := a.BatchQuery(queryAllAgents("io.skupper.router.router", getAddressesFor(routers)))
	if err != nil {
		return err
	}
	for i, records := range results {
		if len(records) == 1 {
			routers[i].Site = GetSiteMetadata(records[0].AsString("metadata"))
		} else {
			return fmt.Errorf("unexpected number of router records: %d", len(records))
		}
	}
	return nil
}

func (a *Agent) getConnectedTo(routers []Router) error {
	results, err := a.BatchQuery(queryAllAgents("io.skupper.router.connection", getAddressesFor(routers)))
	if err != nil {
		return err
	}
	for i, records := range results {
		routers[i].ConnectedTo = []string{}
		for _, r := range records {
			c := asConnection(r)
			if c.Dir == "out" && (c.Role == "edge" || c.Role == "inter-router") {
				routers[i].ConnectedTo = append(routers[i].ConnectedTo, c.Container)
			}
		}
	}
	return nil
}

func getBridgeTypes() []string {
	return []string{
		"io.skupper.router.tcpConnector",
		"io.skupper.router.tcpListener",
		"io.skupper.router.httpConnector",
		"io.skupper.router.httpListener",
	}
}

type TCPEndpointFilter func(*TCPEndpoint) bool

func asTCPEndpoints(records []Record, filter TCPEndpointFilter) []TCPEndpoint {
	endpoints := []TCPEndpoint{}
	for _, record := range records {
		endpoint := asTCPEndpoint(record)
		if filter == nil || filter(&endpoint) {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}

func (a *Agent) getLocalTCPEndpoints(typename string, filter TCPEndpointFilter) ([]TCPEndpoint, error) {
	results, err := a.Query(typename, []string{})
	if err != nil {
		return nil, err
	}
	records := asTCPEndpoints(results, filter)
	return records, nil
}

func (a *Agent) GetConnectorByName(name string) (*Connector, error) {
	results, err := a.Query("io.skupper.router.connector", []string{})
	if err != nil {
		return nil, err
	}
	for _, record := range results {
		result := asConnector(record)

		if result.Name == name {
			return &result, nil
		}
	}

	return nil, nil
}

func (a *Agent) GetSslProfileByName(name string) (*SslProfile, error) {
	results, err := a.Query("io.skupper.router.sslProfile", []string{})
	if err != nil {
		return nil, err
	}
	for _, record := range results {
		result := asSslProfile(record)

		if result.Name == name {
			return &result, nil
		}
	}

	return nil, nil
}

func (a *Agent) GetSslProfiles() (map[string]SslProfile, error) {
	results, err := a.Query("io.skupper.router.sslProfile", []string{})
	if err != nil {
		return nil, err
	}
	profiles := map[string]SslProfile{}
	for _, record := range results {
		profile := asSslProfile(record)
		profiles[profile.Name] = profile
	}

	return profiles, nil
}

func (a *Agent) GetLocalTCPListeners(filter TCPEndpointFilter) ([]TCPEndpoint, error) {
	return a.getLocalTCPEndpoints("io.skupper.router.tcpListener", filter)
}

func (a *Agent) GetLocalTCPConnectors(filter TCPEndpointFilter) ([]TCPEndpoint, error) {
	return a.getLocalTCPEndpoints("io.skupper.router.tcpConnector", filter)
}

func (a *Agent) GetLocalBridgeConfig() (*BridgeConfig, error) {
	config := NewBridgeConfig()

	results, err := a.Query("io.skupper.router.tcpConnector", []string{})
	if err != nil {
		return nil, err
	}
	for _, record := range results {
		config.AddTCPConnector(asTCPEndpoint(record))
	}

	results, err = a.Query("io.skupper.router.tcpListener", []string{})
	if err != nil {
		return nil, err
	}
	for _, record := range results {
		config.AddTCPListener(asTCPEndpoint(record))
	}

	return &config, nil
}

func (a *Agent) UpdateLocalBridgeConfig(changes *BridgeConfigDifference) error {
	for _, deleted := range changes.TCPConnectors.Deleted {
		if err := a.Delete("io.skupper.router.tcpConnector", deleted); err != nil {
			return fmt.Errorf("error deleting tcp connectors: %w", err)
		}
	}
	for _, deleted := range changes.TCPListeners.Deleted {
		if err := a.Delete("io.skupper.router.tcpListener", deleted); err != nil {
			return fmt.Errorf("error deleting tcp listeners: %w", err)
		}
	}
	for _, added := range changes.TCPConnectors.Added {
		if err := a.Create("io.skupper.router.tcpConnector", added.Name, added); err != nil {
			return fmt.Errorf("error adding tcp connectors: %w", err)
		}
	}
	for _, added := range changes.TCPListeners.Added {
		if err := a.Create("io.skupper.router.tcpListener", added.Name, added); err != nil {
			return fmt.Errorf("error adding tcp listeners: %w", err)
		}
	}
	return nil
}

func (a *Agent) GetBridges(routers []Router) ([]BridgeConfig, error) {
	configs := []BridgeConfig{}
	agents := getAddressesFor(routers)
	for _, agent := range agents {
		config := NewBridgeConfig()

		results, err := a.QueryByAgentAddress("io.skupper.router.tcpConnector", []string{}, agent)
		if err != nil {
			return nil, err
		}
		for _, record := range results {
			config.AddTCPConnector(asTCPEndpoint(record))
		}
		results, err = a.QueryByAgentAddress("io.skupper.router.tcpListener", []string{}, agent)
		if err != nil {
			return nil, err
		}
		for _, record := range results {
			config.AddTCPListener(asTCPEndpoint(record))
		}

		configs = append(configs, config)
	}
	return configs, nil
}

const (
	DirectionIn  string = "in"
	DirectionOut string = "out"
)

type TCPConnection struct {
	Name      string `json:"name"`
	Host      string `json:"host"`
	Address   string `json:"address"`
	Direction string `json:"direction"`
	BytesIn   int    `json:"bytesIn"`
	BytesOut  int    `json:"bytesOut"`
	Uptime    uint64 `json:"uptimeSeconds"`
	LastIn    uint64 `json:"lastInSeconds"`
	LastOut   uint64 `json:"lastOutSeconds"`
}

func getTCPConnectionsFromRecords(records []Record) ([]TCPConnection, error) {
	conns := []TCPConnection{}
	for _, record := range records {
		var conn TCPConnection
		if err := convert(record, &conn); err != nil {
			return conns, fmt.Errorf("failed to convert to TCPConnection: %w", err)
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

func (a *Agent) GetTCPConnections(routers []Router) ([][]TCPConnection, error) {
	queries := queryAllAgents("io.skupper.router.tcpConnection", getAddressesFor(routers))
	results, err := a.BatchQuery(queries)
	if err != nil {
		return nil, err
	}
	converted := [][]TCPConnection{}
	for _, records := range results {
		conns, err := getTCPConnectionsFromRecords(records)
		if err != nil {
			return converted, err
		}
		converted = append(converted, conns)
	}
	return converted, nil
}

func (a *Agent) GetLocalTCPConnections() ([]TCPConnection, error) {
	records, err := a.Query("io.skupper.router.tcpConnection", []string{})
	if err != nil {
		return nil, err
	}
	return getTCPConnectionsFromRecords(records)
}

func (a *Agent) getAllEdgeRouters(agents []string) ([]Router, error) {
	edges := []Router{}

	connections, err := a.getConnectionsForAll(agents)
	if err != nil {
		return nil, err
	}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == DirectionIn {
			router := Router{
				ID:      c.Container,
				Edge:    true,
				Address: GetRouterAddress(c.Container, true),
			}
			edges = append(edges, router)
		}
	}
	return edges, nil
}

func (a *Agent) getEdgeRouters(agent string) ([]Router, error) {
	connections, err := a.GetConnectionsFor(agent)
	if err != nil {
		return nil, err
	}
	edges := []Router{}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == DirectionIn {
			router := Router{
				ID:      c.Container,
				Edge:    true,
				Address: GetRouterAddress(c.Container, true),
			}
			edges = append(edges, router)
		}
	}
	return edges, nil
}

func (a *Agent) GetLocalGateways() ([]Router, error) {
	gateways := []Router{}
	connections, err := a.GetConnections()
	if err != nil {
		return gateways, err
	}
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == DirectionIn && isGateway(c.Container) {
			router := Router{
				ID:      c.Container,
				Edge:    true,
				Address: GetRouterAddress(c.Container, true),
			}
			gateways = append(gateways, router)
		}
	}
	err = a.getSiteIDs(gateways)
	return gateways, err
}

func (a *Agent) GetLocalRouter() (*Router, error) {
	records, err := a.Query("io.skupper.router.router", []string{})
	if err != nil {
		return nil, err
	}
	if len(records) == 1 {
		return asRouter(records[0]), nil
	}
	return nil, fmt.Errorf("unexpected number of router records: %d", len(records))
}

func (a *Agent) isEdgeRouter() bool {
	return a.local.Edge
}

func (a *Agent) getInteriorAddressForUplink() (string, error) {
	connections, err := a.GetConnections()
	if err != nil {
		return "", err
	}
	return GetInteriorAddressForUplink(connections)
}

func GetInteriorAddressForUplink(connections []Connection) (string, error) {
	for _, c := range connections {
		if c.Role == "edge" && c.Dir == "out" {
			return GetRouterAgentAddress(c.Container, false), nil
		}
	}
	return "", errors.New("could not find uplink connection")
}

type ConnectorStatus struct {
	Name        string
	Host        string
	Port        string
	Role        string
	Cost        int
	Status      string
	Description string
}

func asConnectorStatus(record Record) ConnectorStatus {
	return ConnectorStatus{
		Name:        record.AsString("name"),
		Host:        record.AsString("host"),
		Port:        record.AsString("port"),
		Role:        record.AsString("role"),
		Cost:        record.AsInt("cost"),
		Status:      record.AsString("connectionStatus"),
		Description: record.AsString("connectionMsg"),
	}
}

func asConnector(record Record) Connector {
	return Connector{
		Name:           record.AsString("name"),
		Host:           record.AsString("host"),
		Port:           record.AsString("port"),
		RouteContainer: record.AsBool("routeContainer"),
		VerifyHostname: record.AsBool("verifyHostname"),
		SslProfile:     record.AsString("sslProfile"),
	}
}

func asInt32(s string) int32 {
	ival, _ := strconv.Atoi(s)
	return int32(ival)
}

func asListener(record Record) Listener {
	return Listener{
		Name:             record.AsString("name"),
		Role:             asRole(record.AsString("role")),
		Host:             record.AsString("host"),
		Port:             asInt32(record.AsString("port")),
		Cost:             int32(record.AsInt("cost")),
		LinkCapacity:     int32(record.AsInt("linkCapacity")),
		AuthenticatePeer: record.AsBool("authenticatePeer"),
		SaslMechanisms:   record.AsString("saslMechanisms"),
		RouteContainer:   record.AsBool("routeContainer"),
		HTTP:             record.AsBool("http"),
		HTTPRootDir:      record.AsString("httpRootDir"),
		Websockets:       record.AsBool("websockets"),
		Healthz:          record.AsBool("healthz"),
		Metrics:          record.AsBool("metrics"),
		SslProfile:       record.AsString("sslProfile"),
		MaxFrameSize:     record.AsInt("maxFrameSize"),
		MaxSessionFrames: record.AsInt("maxSessionFrames"),
	}
}

func asSslProfile(record Record) SslProfile {
	return SslProfile{
		Name:           record.AsString("name"),
		CertFile:       record.AsString("certFile"),
		PrivateKeyFile: record.AsString("privateKeyFile"),
		CaCertFile:     record.AsString("caCertFile"),
	}
}

func (a *Agent) UpdateConnectorConfig(changes *ConnectorDifference) error {
	for _, deleted := range changes.Deleted {
		if err := a.Delete("io.skupper.router.connector", deleted.Name); err != nil {
			return fmt.Errorf("error deleting connectors: %w", err)
		}
	}

	for _, added := range changes.Added {
		if len(added.Host) == 0 {
			return errors.New("no host specified while creating a connector")
		}

		if len(added.Port) == 0 {
			return errors.New("no port specified while creating a connector")
		}

		if len(added.SslProfile) > 0 {
			sslProfile, err := a.GetSslProfileByName(added.SslProfile)
			if err != nil {
				return err
			}

			if sslProfile.CaCertFile != "" {
				_, err = os.Stat(sslProfile.CaCertFile)
				if err != nil {
					return err
				}
			}

			if sslProfile.CertFile != "" {
				_, err = os.Stat(sslProfile.CertFile)
				if err != nil {
					return err
				}
			}

			if sslProfile.PrivateKeyFile != "" {
				_, err = os.Stat(sslProfile.PrivateKeyFile)
				if err != nil {
					return err
				}
			}
		}

		if err := a.Create("io.skupper.router.connector", added.Name, added); err != nil {
			return fmt.Errorf("error adding connectors: %w", err)
		}
	}

	return nil
}

func (a *Agent) GetLocalConnectorStatus() (map[string]ConnectorStatus, error) {
	results, err := a.Query("io.skupper.router.connector", []string{})
	if err != nil {
		return nil, err
	}
	connectors := map[string]ConnectorStatus{}
	for _, record := range results {
		c := asConnectorStatus(record)
		connectors[c.Name] = c
	}
	return connectors, nil
}

func (a *Agent) GetLocalConnectors() (map[string]Connector, error) {
	results, err := a.Query("io.skupper.router.connector", []string{})
	if err != nil {
		return nil, err
	}
	connectors := map[string]Connector{}
	for _, record := range results {
		c := asConnector(record)
		connectors[c.Name] = c
	}
	return connectors, nil
}

func (a *Agent) UpdateListenerConfig(changes *ListenerDifference) error {
	for _, deleted := range changes.Deleted {
		if err := a.Delete("io.skupper.router.listener", deleted.Name); err != nil {
			return fmt.Errorf("error deleting listeners: %w", err)
		}
	}

	for _, added := range changes.Added {
		if err := a.Create("io.skupper.router.listener", added.Name, added); err != nil {
			return fmt.Errorf("error adding listeners: %w", err)
		}
	}

	return nil
}

func (a *Agent) GetLocalListeners() (map[string]Listener, error) {
	results, err := a.Query("io.skupper.router.listener", []string{})
	if err != nil {
		return nil, err
	}
	listeners := map[string]Listener{}
	for _, record := range results {
		l := asListener(record)
		listeners[l.Name] = l
	}
	return listeners, nil
}

func (a *Agent) Request(request *Request) (*Response, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	requestMsg := amqp.Message{
		Properties: &amqp.MessageProperties{
			To:      request.Address,
			Subject: request.Type,
			ReplyTo: a.receiver.Address(),
		},
		ApplicationProperties: map[string]any{},
		Value:                 nil,
	}
	if request.Body != "" {
		requestMsg.Value = request.Body
	}
	for k, v := range request.Properties {
		requestMsg.ApplicationProperties[k] = v
	}
	requestMsg.ApplicationProperties[VersionProperty] = request.Version

	err := a.anonymous.Send(ctx, &requestMsg)
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("could not send %s request: %w", request.Type, err)
	}
	responseMsg, err := a.receiver.Receive(ctx)
	if err != nil {
		_ = a.Close()
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}
	_ = responseMsg.Accept()

	response := Response{
		Type: responseMsg.Properties.Subject,
	}
	for k, v := range responseMsg.ApplicationProperties {
		if k == VersionProperty {
			if version, ok := v.(string); ok {
				response.Version = version
			}
		} else {
			response.Properties[k] = v
		}
	}
	if body, ok := responseMsg.Value.(string); ok {
		response.Body = body
	}
	return &response, nil
}

func (r *Router) IsGateway() bool {
	return isGateway(r.ID)
}

func isGateway(routerID string) bool {
	return strings.HasPrefix(routerID, "skupper-gateway-")
}

func GetSiteNameForGateway(gateway *Router) string {
	return strings.TrimPrefix(gateway.ID, "skupper-gateway-")
}

func (a *Agent) CreateSslProfile(profile SslProfile) error {
	result, err := a.GetSslProfileByName(profile.Name)
	if err != nil {
		return err
	}

	// Trying to create a ssl profile that already exists will generate an error in the router.
	if result != nil {
		return nil
	}

	if err := a.Create("io.skupper.router.sslProfile", profile.Name, profile); err != nil {
		return fmt.Errorf("error adding SSL Profile: %w", err)
	}

	return nil
}

func (a *Agent) ReloadSslProfile(name string) error {
	profile, err := a.GetSslProfileByName(name)
	if err != nil {
		return err
	}

	// A profile is expected to be returned
	if profile == nil {
		return fmt.Errorf("no SSL Profile with name %s found", name)
	}

	if err := a.Update("io.skupper.router.sslProfile", profile.Name, profile); err != nil {
		return fmt.Errorf("error updating SSL Profile: %w", err)
	}

	return nil
}

func ConnectedSitesInfo(selfID string, routers []Router) types.TransportConnectedSites {
	var connectedSites types.TransportConnectedSites
	var self *Router
	for _, r := range routers {
		if r.Site.ID == selfID {
			self = &r
			break
		}
	}
	if self == nil {
		return connectedSites
	}
	for _, r := range routers {
		if r.Edge && len(r.ConnectedTo) > 1 {
			connectedSites.Warnings = append(connectedSites.Warnings, "There are edge uplinks to distinct networks, please verify topology (connected counts may not be accurate).")
		}
		if utils.StringSliceContains(r.ConnectedTo, self.ID) {
			connectedSites.Direct++
		}
	}
	connectedSites.Total = len(routers) - 1
	connectedSites.Direct += len(self.ConnectedTo)
	connectedSites.Indirect = connectedSites.Total - connectedSites.Direct
	return connectedSites
}
