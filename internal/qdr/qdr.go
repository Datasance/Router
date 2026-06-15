package qdr

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path"
	"reflect"
	"strconv"
	"strings"

	types "github.com/eclipse-iofog/router/internal/resources/skuppertypes"
)

type RouterConfig struct {
	Metadata    RouterMetadata
	SslProfiles map[string]SslProfile
	Listeners   map[string]Listener
	Connectors  map[string]Connector
	Addresses   map[string]Address
	LogConfig   map[string]LogConfig
	SiteConfig  *SiteConfig
	Bridges     BridgeConfig
}

type RouterConfigHandler interface {
	GetRouterConfig() (*RouterConfig, error)
	SaveRouterConfig(*RouterConfig) error
	RemoveRouterConfig() error
}

type TCPEndpointMap map[string]TCPEndpoint

type BridgeConfig struct {
	TCPListeners  TCPEndpointMap
	TCPConnectors TCPEndpointMap
}

func InitialConfig(id string, siteID string, version string, edge bool, helloAge int) RouterConfig {
	config := RouterConfig{
		Metadata: RouterMetadata{
			ID:                 id,
			HelloMaxAgeSeconds: strconv.Itoa(helloAge),
			Metadata:           getSiteMetadataString(siteID, version),
		},
		Addresses:   map[string]Address{},
		SslProfiles: map[string]SslProfile{},
		Listeners:   map[string]Listener{},
		Connectors:  map[string]Connector{},
		LogConfig:   map[string]LogConfig{},
		Bridges: BridgeConfig{
			TCPListeners:  map[string]TCPEndpoint{},
			TCPConnectors: map[string]TCPEndpoint{},
		},
	}
	if edge {
		config.Metadata.Mode = ModeEdge
	} else {
		config.Metadata.Mode = ModeInterior
	}
	return config
}

func (r *RouterConfig) AddHealthAndMetricsListener(port int32) {
	r.AddListener(Listener{
		Port:        port,
		Role:        "normal",
		HTTP:        true,
		HTTPRootDir: "disabled",
		Websockets:  false,
		Healthz:     true,
		Metrics:     true,
	})
}

func NewBridgeConfig() BridgeConfig {
	return BridgeConfig{
		TCPListeners:  map[string]TCPEndpoint{},
		TCPConnectors: map[string]TCPEndpoint{},
	}
}

func NewBridgeConfigCopy(src BridgeConfig) BridgeConfig {
	newBridges := NewBridgeConfig()
	for k, v := range src.TCPListeners {
		newBridges.TCPListeners[k] = v
	}
	for k, v := range src.TCPConnectors {
		newBridges.TCPConnectors[k] = v
	}
	return newBridges
}

func (r *RouterConfig) AddListener(l Listener) bool {
	if l.Name == "" {
		l.Name = fmt.Sprintf("%s@%d", l.Host, l.Port)
	}
	if original, ok := r.Listeners[l.Name]; ok && original == l {
		return false
	}
	r.Listeners[l.Name] = l
	return true
}

func (r *RouterConfig) RemoveListener(name string) (bool, Listener) {
	c, ok := r.Listeners[name]
	if ok {
		delete(r.Listeners, name)
		return true, c
	}
	return false, Listener{}
}

func (r *RouterConfig) AddConnector(c Connector) bool {
	if original, ok := r.Connectors[c.Name]; ok && original == c {
		return false
	}
	r.Connectors[c.Name] = c
	return true
}

func (r *RouterConfig) RemoveConnector(name string) (bool, Connector) {
	c, ok := r.Connectors[name]
	if ok {
		delete(r.Connectors, name)
		return true, c
	}
	return false, Connector{}
}

func (r *RouterConfig) IsEdge() bool {
	return r.Metadata.Mode == ModeEdge
}

// ConfigureSslProfile builds an SslProfile with file paths under the given base path.
// For the default SSL profile directory, use config.GetSSLProfilePath().
func ConfigureSslProfile(name string, basePath string, clientAuth bool) SslProfile {
	profile := SslProfile{
		Name:       name,
		CaCertFile: path.Join(basePath, name, "ca.crt"),
	}
	if clientAuth {
		profile.CertFile = path.Join(basePath, name, "tls.crt")
		profile.PrivateKeyFile = path.Join(basePath, name, "tls.key")
	}
	return profile
}

func (r *RouterConfig) AddSslProfile(s SslProfile) bool {
	if original, ok := r.SslProfiles[s.Name]; ok && original == s {
		return false
	}
	r.SslProfiles[s.Name] = s
	return true
}

func (r *RouterConfig) RemoveSslProfile(name string) bool {
	_, ok := r.SslProfiles[name]
	if ok {
		delete(r.SslProfiles, name)
		return true
	}
	return false
}

func (r *RouterConfig) RemoveUnreferencedSslProfiles() bool {
	unreferenced := r.UnreferencedSslProfiles()
	changed := false
	for _, profile := range unreferenced {
		if r.RemoveSslProfile(profile.Name) {
			changed = true
		}
	}
	return changed
}

func (r *RouterConfig) UnreferencedSslProfiles() map[string]SslProfile {
	results := map[string]SslProfile{}
	for _, profile := range r.SslProfiles {
		results[profile.Name] = profile
	}
	// remove any that are referenced
	for _, o := range r.Listeners {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Connectors {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Bridges.TCPListeners {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Bridges.TCPConnectors {
		delete(results, o.SslProfile)
	}

	return results
}

func (r *RouterConfig) AddAddress(a Address) {
	r.Addresses[a.Prefix] = a
}

func (r *RouterConfig) AddTCPConnector(e TCPEndpoint) {
	r.Bridges.AddTCPConnector(e)
}

func (r *RouterConfig) RemoveTCPConnector(name string) (bool, TCPEndpoint) {
	return r.Bridges.RemoveTCPConnector(name)
}

func (r *RouterConfig) AddTCPListener(e TCPEndpoint) {
	r.Bridges.AddTCPListener(e)
}

func (r *RouterConfig) RemoveTCPListener(name string) (bool, TCPEndpoint) {
	return r.Bridges.RemoveTCPListener(name)
}

func (r *RouterConfig) UpdateBridgeConfig(desired BridgeConfig) bool {
	if reflect.DeepEqual(r.Bridges, desired) {
		return false
	}
	r.Bridges = desired
	return true
}

func (r *RouterConfig) GetSiteMetadata() SiteMetadata {
	return GetSiteMetadata(r.Metadata.Metadata)
}

func (r *RouterConfig) SetSiteMetadata(site *SiteMetadata) {
	r.Metadata.Metadata = getSiteMetadataString(site.ID, site.Version)
}

func (bc *BridgeConfig) AddTCPConnector(e TCPEndpoint) {
	bc.TCPConnectors[e.Name] = e
}

func (bc *BridgeConfig) RemoveTCPConnector(name string) (bool, TCPEndpoint) {
	tc, ok := bc.TCPConnectors[name]
	if ok {
		delete(bc.TCPConnectors, name)
		return true, tc
	}
	return false, TCPEndpoint{}
}

func (bc *BridgeConfig) AddTCPListener(e TCPEndpoint) {
	bc.TCPListeners[e.Name] = e
}

func (bc *BridgeConfig) RemoveTCPListener(name string) (bool, TCPEndpoint) {
	tc, ok := bc.TCPListeners[name]
	if ok {
		delete(bc.TCPListeners, name)
		return true, tc
	}
	return false, TCPEndpoint{}
}

func GetTCPConnectors(bridges []BridgeConfig) []TCPEndpoint {
	connectors := []TCPEndpoint{}
	for _, bridge := range bridges {
		for _, connector := range bridge.TCPConnectors {
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func (r *RouterConfig) SetLogLevel(module string, level string) bool {
	if level != "" {
		config := LogConfig{
			Module: module,
			Enable: level,
		}
		if module == "" {
			config.Module = "DEFAULT"
		}
		if !strings.HasSuffix(level, "+") {
			config.Enable = level + "+"
		}
		if r.LogConfig == nil {
			r.LogConfig = map[string]LogConfig{}
		}
		if r.LogConfig[config.Module] != config {
			r.LogConfig[config.Module] = config
			return true
		}
	}
	return false
}

func (r *RouterConfig) SetLogLevels(levels map[string]string) bool {
	keys := map[string]bool{}
	for k := range levels {
		if k == "" {
			k = "DEFAULT"
		}
		keys[k] = true
	}
	changed := false
	for name, level := range levels {
		if r.SetLogLevel(name, level) {
			changed = true
		}
	}
	for key := range r.LogConfig {
		if _, ok := keys[key]; !ok {
			delete(r.LogConfig, key)
			changed = true
		}
	}
	return changed
}

type Role string

const (
	RoleInterRouter Role = "inter-router"
	RoleEdge        Role = "edge"
	RoleNormal      Role = "normal"
	RoleDefault     Role = ""
)

func asRole(name string) Role {
	if name == "edge" {
		return RoleEdge
	}
	if name == "inter-router" {
		return RoleInterRouter
	}
	if name == "normal" {
		return RoleNormal
	}
	return RoleDefault
}

func GetRole(name string) Role {
	switch name {
	case "edge":
		return RoleEdge
	case "normal":
		return RoleNormal
	}
	return RoleInterRouter
}

type Mode string

const (
	ModeInterior Mode = "interior"
	ModeEdge     Mode = "edge"
)

type RouterMetadata struct {
	ID                  string `json:"id,omitempty"`
	Mode                Mode   `json:"mode,omitempty"`
	HelloMaxAgeSeconds  string `json:"helloMaxAgeSeconds,omitempty"`
	DataConnectionCount string `json:"dataConnectionCount,omitempty"`
	Metadata            string `json:"metadata,omitempty"`
}

type SslProfile struct {
	Name           string `json:"name,omitempty"`
	CertFile       string `json:"certFile,omitempty"`
	PrivateKeyFile string `json:"privateKeyFile,omitempty"`
	CaCertFile     string `json:"caCertFile,omitempty"`
}

func (p SslProfile) toRecord() Record {
	result := make(map[string]any)
	if p.Name != "" {
		result["name"] = p.Name
	}
	if p.CertFile != "" {
		result["certFile"] = p.CertFile
	}
	if p.PrivateKeyFile != "" {
		result["privateKeyFile"] = p.PrivateKeyFile
	}
	if p.CaCertFile != "" {
		result["caCertFile"] = p.CaCertFile
	}
	return result
}

type LogConfig struct {
	Module string `json:"module"`
	Enable string `json:"enable"`
}

type Listener struct {
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	Role             Role   `json:"role,omitempty" yaml:"role,omitempty"`
	Host             string `json:"host,omitempty" yaml:"host,omitempty"`
	Port             int32  `json:"port" yaml:"port,omitempty"`
	RouteContainer   bool   `json:"routeContainer,omitempty" yaml:"route-container,omitempty"`
	HTTP             bool   `json:"http,omitempty" yaml:"http,omitempty"`
	Cost             int32  `json:"cost,omitempty" yaml:"cost,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty" yaml:"ssl-profile,omitempty"`
	SaslMechanisms   string `json:"saslMechanisms,omitempty" yaml:"sasl-mechanisms,omitempty"`
	AuthenticatePeer bool   `json:"authenticatePeer,omitempty" yaml:"authenticate-peer,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty" yaml:"link-capacity,omitempty"`
	HTTPRootDir      string `json:"httpRootDir,omitempty" yaml:"http-rootdir,omitempty"`
	Websockets       bool   `json:"websockets,omitempty" yaml:"web-sockets,omitempty"`
	Healthz          bool   `json:"healthz,omitempty" yaml:"healthz,omitempty"`
	Metrics          bool   `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	MaxFrameSize     int    `json:"maxFrameSize,omitempty" yaml:"max-frame-size,omitempty"`
	MaxSessionFrames int    `json:"maxSessionFrames,omitempty" yaml:"max-session-frames,omitempty"`
}

func (listener Listener) toRecord() Record {
	record := map[string]any{}
	record["name"] = listener.Name
	record["role"] = string(listener.Role)
	record["host"] = listener.Host
	record["port"] = strconv.Itoa(int(listener.Port))
	if listener.Cost > 0 {
		record["cost"] = listener.Cost
	}
	if listener.LinkCapacity > 0 {
		record["linkCapacity"] = listener.LinkCapacity
	}
	if len(listener.SslProfile) > 0 {
		record["sslProfile"] = listener.SslProfile
	}
	if listener.AuthenticatePeer {
		record["authenticatePeer"] = listener.AuthenticatePeer
	}
	if len(listener.SaslMechanisms) > 0 {
		record["saslMechanisms"] = listener.SaslMechanisms
	}
	if listener.MaxFrameSize > 0 {
		record["maxFrameSize"] = listener.MaxFrameSize
	}
	if listener.MaxSessionFrames > 0 {
		record["maxSessionFrames"] = listener.MaxSessionFrames
	}
	if listener.RouteContainer {
		record["routeContainer"] = listener.RouteContainer
	}
	if listener.HTTP {
		record["http"] = listener.HTTP
	}
	if len(listener.HTTPRootDir) > 0 {
		record["httpRootDir"] = listener.HTTPRootDir
	}
	if listener.Websockets {
		record["websockets"] = listener.Websockets
	}
	if listener.Healthz {
		record["healthz"] = listener.Healthz
	}
	if listener.Metrics {
		record["metrics"] = listener.Metrics
	}

	return record
}
func (listener *Listener) SetMaxFrameSize(value int) {
	listener.MaxFrameSize = value
}

func (listener *Listener) SetMaxSessionFrames(value int) {
	listener.MaxSessionFrames = value
}

type Connector struct {
	Name             string `json:"name,omitempty"`
	Role             Role   `json:"role,omitempty"`
	Host             string `json:"host"`
	Port             string `json:"port"`
	RouteContainer   bool   `json:"routeContainer,omitempty"`
	Cost             int32  `json:"cost,omitempty"`
	VerifyHostname   bool   `json:"verifyHostname,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty"`
	MaxFrameSize     int    `json:"maxFrameSize,omitempty"`
	MaxSessionFrames int    `json:"maxSessionFrames,omitempty"`
}

func (connector Connector) toRecord() Record {
	record := map[string]any{}
	record["name"] = connector.Name
	record["role"] = string(connector.Role)
	record["host"] = connector.Host
	record["port"] = connector.Port
	if connector.Cost > 0 {
		record["cost"] = connector.Cost
	}
	if len(connector.SslProfile) > 0 {
		record["sslProfile"] = connector.SslProfile
	}
	if connector.MaxFrameSize > 0 {
		record["maxFrameSize"] = connector.MaxFrameSize
	}
	if connector.MaxSessionFrames > 0 {
		record["maxSessionFrames"] = connector.MaxSessionFrames
	}
	return record
}

func (connector *Connector) SetMaxFrameSize(value int) {
	connector.MaxFrameSize = value
}

func (connector *Connector) SetMaxSessionFrames(value int) {
	connector.MaxSessionFrames = value
}

type Distribution string

const (
	DistributionBalanced  Distribution = "balanced"
	DistributionMulticast Distribution = "multicast"
	DistributionClosest   Distribution = "closest"
)

type Address struct {
	Prefix       string `json:"prefix,omitempty"`
	Distribution string `json:"distribution,omitempty"`
}

type TCPEndpoint struct {
	Name           string `json:"name,omitempty"`
	Host           string `json:"host,omitempty"`
	Port           string `json:"port,omitempty"`
	Address        string `json:"address,omitempty"`
	SiteID         string `json:"siteID,omitempty"`
	SslProfile     string `json:"sslProfile,omitempty"`
	VerifyHostname *bool  `json:"verifyHostname,omitempty"`
	ProcessID      string `json:"processId,omitempty"`
}

func (e TCPEndpoint) toRecord() Record {
	result := make(map[string]any)
	if e.Name != "" {
		result["name"] = e.Name
	}
	if e.Host != "" {
		result["host"] = e.Host
	}
	if e.Port != "" {
		result["port"] = e.Port
	}
	if e.Address != "" {
		result["address"] = e.Address
	}
	if e.SiteID != "" {
		result["siteID"] = e.SiteID
	}
	if e.SslProfile != "" {
		result["sslProfile"] = e.SslProfile
	}
	if e.VerifyHostname != nil {
		result["verifyHostname"] = e.VerifyHostname
	}
	if e.ProcessID != "" {
		result["processId"] = e.ProcessID
	}
	return result
}

type SiteConfig struct {
	Name      string `json:"name,omitempty"`
	Location  string `json:"location,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
}

func convert(from any, to any) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, to)
	if err != nil {
		return err
	}
	return nil
}

func RouterConfigEquals(actual, desired string) bool {
	actualConfig, err := UnmarshalRouterConfig(actual)
	if err != nil {
		return false
	}
	desiredConfig, err := UnmarshalRouterConfig(desired)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(actualConfig, desiredConfig)
}

func UnmarshalRouterConfig(config string) (RouterConfig, error) {
	result := RouterConfig{
		Metadata:    RouterMetadata{},
		Addresses:   map[string]Address{},
		SslProfiles: map[string]SslProfile{},
		Listeners:   map[string]Listener{},
		Connectors:  map[string]Connector{},
		LogConfig:   map[string]LogConfig{},
		Bridges: BridgeConfig{
			TCPListeners:  map[string]TCPEndpoint{},
			TCPConnectors: map[string]TCPEndpoint{},
		},
	}
	var obj any
	err := json.Unmarshal([]byte(config), &obj)
	if err != nil {
		return result, err
	}
	elements, ok := obj.([]any)
	if !ok {
		return result, fmt.Errorf("invalid JSON for router configuration, expected array at top level got %#v", obj)
	}
	for _, e := range elements {
		element, ok := e.([]any)
		if !ok || len(element) != 2 {
			return result, fmt.Errorf("invalid JSON for router configuration, expected array with type and value got %#v", e)
		}
		entityType, ok := element[0].(string)
		if !ok {
			return result, fmt.Errorf("invalid JSON for router configuration, expected entity type as string got %#v", element[0])
		}
		switch entityType {
		case "router":
			metadata := RouterMetadata{}
			err = convert(element[1], &metadata)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Metadata = metadata
		case "address":
			address := Address{}
			err = convert(element[1], &address)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Addresses[address.Prefix] = address
		case "connector":
			connector := Connector{}
			err = convert(element[1], &connector)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Connectors[connector.Name] = connector
		case "listener":
			listener := Listener{}
			err = convert(element[1], &listener)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Listeners[listener.Name] = listener
		case "sslProfile":
			sslProfile := SslProfile{}
			err = convert(element[1], &sslProfile)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.SslProfiles[sslProfile.Name] = sslProfile
		case "log":
			logConfig := LogConfig{}
			err = convert(element[1], &logConfig)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.LogConfig[logConfig.Module] = logConfig
		case "site":
			siteConfig := &SiteConfig{}
			err = convert(element[1], siteConfig)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.SiteConfig = siteConfig
		case "tcpConnector":
			connector := TCPEndpoint{}
			err = convert(element[1], &connector)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.TCPConnectors[connector.Name] = connector
		case "tcpListener":
			listener := TCPEndpoint{}
			err = convert(element[1], &listener)
			if err != nil {
				return result, fmt.Errorf("invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.TCPListeners[listener.Name] = listener
		default:
		}
	}
	return result, nil
}

func MarshalRouterConfig(config RouterConfig) (string, error) {
	elements := [][]any{}
	tuple := []any{
		"router",
		config.Metadata,
	}
	elements = append(elements, tuple)
	for _, e := range config.SslProfiles {
		tuple := []any{
			"sslProfile",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Connectors {
		tuple := []any{
			"connector",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Listeners {
		tuple := []any{
			"listener",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Addresses {
		tuple := []any{
			"address",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.TCPConnectors {
		tuple := []any{
			"tcpConnector",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.TCPListeners {
		tuple := []any{
			"tcpListener",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.LogConfig {
		tuple := []any{
			"log",
			e,
		}
		elements = append(elements, tuple)
	}
	if config.SiteConfig != nil {
		tuple := []any{
			"site",
			*config.SiteConfig,
		}
		elements = append(elements, tuple)
	}
	data, err := json.MarshalIndent(elements, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func AsConfigMapData(config string) map[string]string {
	return map[string]string{
		types.TransportConfigFile: config,
	}
}

func (r *RouterConfig) AsConfigMapData() (map[string]string, error) {
	result := map[string]string{}
	marshaled, err := MarshalRouterConfig(*r)
	if err != nil {
		return result, err
	}
	result[types.TransportConfigFile] = marshaled
	return result, nil
}

type ListenerPredicate func(Listener) bool

func IsNotNormalListener(l Listener) bool {
	return l.Role != "normal" && l.Role != ""
}

func FilterListeners(in map[string]Listener, predicate ListenerPredicate) map[string]Listener {
	results := map[string]Listener{}
	for key, listener := range in {
		if predicate(listener) {
			results[key] = listener
		}
	}
	return results
}

func (r *RouterConfig) GetMatchingListeners(predicate ListenerPredicate) map[string]Listener {
	return FilterListeners(r.Listeners, predicate)
}

type ConnectorDifference struct {
	Deleted          []Connector
	Added            []Connector
	AddedSslProfiles map[string]SslProfile
}

type TCPEndpointDifference struct {
	Deleted []string
	Added   []TCPEndpoint
}

type BridgeConfigDifference struct {
	TCPListeners       TCPEndpointDifference
	TCPConnectors      TCPEndpointDifference
	AddedSslProfiles   []string
	DeletedSSlProfiles []string
}

func isAddrAny(host string) bool {
	ip := net.ParseIP(host)
	return ip.Equal(net.IPv4zero) || ip.Equal(net.IPv6zero)
}

func equivalentHost(a string, b string) bool {
	switch a {
	case b:
		return true
	case "":
		return isAddrAny(b)
	default:
		if b == "" {
			return isAddrAny(a)
		}
		return false
	}
}

func (e TCPEndpoint) equivalentVerifyHostname(b TCPEndpoint) bool {
	if e.VerifyHostname == nil {
		return b.VerifyHostname == nil || *b.VerifyHostname
	}
	if b.VerifyHostname == nil {
		return e.VerifyHostname == nil || *e.VerifyHostname
	}
	return *e.VerifyHostname == *b.VerifyHostname
}

func (e TCPEndpoint) Equivalent(b TCPEndpoint) bool {
	if !equivalentHost(e.Host, b.Host) || e.Port != b.Port || e.Address != b.Address ||
		e.SiteID != b.SiteID || e.ProcessID != b.ProcessID || !e.equivalentVerifyHostname(b) {
		return false
	}
	return true
}

func (a TCPEndpointMap) Difference(b TCPEndpointMap) TCPEndpointDifference {
	result := TCPEndpointDifference{}
	for key, v1 := range b {
		v2, ok := a[key]
		if !ok {
			result.Added = append(result.Added, v1)
		} else if !v1.Equivalent(v2) {
			result.Deleted = append(result.Deleted, v1.Name)
			result.Added = append(result.Added, v1)
		}
	}
	for key, v1 := range a {
		_, ok := b[key]
		if !ok {
			result.Deleted = append(result.Deleted, v1.Name)
		}
	}
	return result
}

func (bc *BridgeConfig) Difference(b *BridgeConfig) *BridgeConfigDifference {
	result := BridgeConfigDifference{
		TCPConnectors: bc.TCPConnectors.Difference(b.TCPConnectors),
		TCPListeners:  bc.TCPListeners.Difference(b.TCPListeners),
	}

	result.AddedSslProfiles, result.DeletedSSlProfiles = getSslProfilesDifference(bc, b)

	return &result
}

type AddedSslProfiles []string
type DeletedSslProfiles []string

func getSslProfilesDifference(before *BridgeConfig, desired *BridgeConfig) (AddedSslProfiles, DeletedSslProfiles) {
	var addedProfiles AddedSslProfiles
	var deletedProfiles DeletedSslProfiles

	originalSslConfig := make(map[string]string)
	newSslConfig := make(map[string]string)

	for _, tcpConnector := range before.TCPConnectors {
		originalSslConfig[tcpConnector.SslProfile] = tcpConnector.SslProfile
	}
	for _, tcpListener := range before.TCPListeners {
		originalSslConfig[tcpListener.SslProfile] = tcpListener.SslProfile
	}

	for _, tcpConnector := range desired.TCPConnectors {
		newSslConfig[tcpConnector.SslProfile] = tcpConnector.SslProfile
	}
	for _, tcpListener := range desired.TCPListeners {
		newSslConfig[tcpListener.SslProfile] = tcpListener.SslProfile
	}

	// Auto-generated Skupper certs will be deleted if they are not used in the desired configuration
	for key, name := range originalSslConfig {
		_, ok := newSslConfig[key]

		if !ok && isGeneratedBySkupper(name) {
			deletedProfiles = append(deletedProfiles, name)
		}
	}

	// New profiles associated with http or tcp connectors/listeners will be created in the router
	for key, name := range newSslConfig {
		_, ok := originalSslConfig[key]

		if !ok && name != types.ServiceClientSecret {
			addedProfiles = append(addedProfiles, name)
		}
	}

	return addedProfiles, deletedProfiles
}

func isGeneratedBySkupper(name string) bool {
	return strings.HasPrefix(name, types.SkupperServiceCertPrefix) && name != types.ServiceClientSecret
}

func (a *TCPEndpointDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func (a *BridgeConfigDifference) Empty() bool {
	return a.TCPConnectors.Empty() && a.TCPListeners.Empty()
}

func (a *BridgeConfigDifference) Print() {
	log.Printf("TCPConnectors added=%v, deleted=%v", a.TCPConnectors.Added, a.TCPConnectors.Deleted)
	log.Printf("TCPListeners added=%v, deleted=%v", a.TCPListeners.Added, a.TCPListeners.Deleted)
	log.Printf("SslProfiles added=%v, deleted=%v", a.AddedSslProfiles, a.DeletedSSlProfiles)
}

func ConnectorsDifference(actual map[string]Connector, desired *RouterConfig, ignorePrefix *string) *ConnectorDifference {
	result := ConnectorDifference{}
	result.AddedSslProfiles = make(map[string]SslProfile)
	for key, v1 := range desired.Connectors {
		_, ok := actual[key]
		if !ok {
			result.Added = append(result.Added, v1)
			result.AddedSslProfiles[v1.SslProfile] = desired.SslProfiles[v1.SslProfile]
		}
	}
	for key, v1 := range actual {
		_, ok := desired.Connectors[key]

		allowedToDelete := true
		if ignorePrefix != nil && len(*ignorePrefix) > 0 {
			allowedToDelete = !strings.HasPrefix(v1.Name, *ignorePrefix)
		}

		if !ok && allowedToDelete {
			result.Deleted = append(result.Deleted, v1)
		}
	}
	return &result
}

func (a *ConnectorDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

type ListenerDifference struct {
	Deleted []Listener
	Added   []Listener
}

func (listener Listener) Equivalent(actual Listener) bool {
	return listener.Name == actual.Name &&
		listener.Role == actual.Role &&
		listener.Host == actual.Host &&
		listener.Port == actual.Port &&
		listener.RouteContainer == actual.RouteContainer &&
		listener.HTTP == actual.HTTP &&
		listener.SslProfile == actual.SslProfile &&
		listener.SaslMechanisms == actual.SaslMechanisms &&
		listener.AuthenticatePeer == actual.AuthenticatePeer &&
		(listener.Cost == 0 || listener.Cost == actual.Cost) &&
		(listener.MaxFrameSize == 0 || listener.MaxFrameSize == actual.MaxFrameSize) &&
		(listener.MaxSessionFrames == 0 || listener.MaxSessionFrames == actual.MaxSessionFrames) &&
		(listener.LinkCapacity == 0 || listener.LinkCapacity == actual.LinkCapacity) &&
		(listener.HTTPRootDir == "" || listener.HTTPRootDir == actual.HTTPRootDir)
	// Skip check for Websockets, Healthz and Metrics as they are
	// always coming back as true at present and are not used where
	// this method is required at present.
}

func ListenersDifference(actual map[string]Listener, desired map[string]Listener) *ListenerDifference {
	result := ListenerDifference{}
	for key, desiredValue := range desired {
		if actualValue, ok := actual[key]; ok {
			if !desiredValue.Equivalent(actualValue) {
				log.Printf("Listener definition does not match. Have %v want %v", actualValue, desiredValue)
				// handle change as delete then add, so it also works over management protocol
				result.Deleted = append(result.Deleted, desiredValue)
				result.Added = append(result.Added, desiredValue)
			}
		} else {
			result.Added = append(result.Added, desiredValue)
		}
	}
	for key, value := range actual {
		if _, ok := desired[key]; !ok {
			result.Deleted = append(result.Deleted, value)
		}
	}
	return &result
}

func (a *ListenerDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

// func GetRouterConfigForHeadlessProxy(definition types.ServiceInterface, siteID string, version string, namespace string, profilePath string) (string, error) {
// 	config := InitialConfig("${HOSTNAME}-"+siteID, siteID, version, true, 3)
// 	// add edge-connector
// 	config.AddSslProfile(ConfigureSslProfile(types.InterRouterProfile, profilePath, true))
// 	config.AddConnector(Connector{
// 		Name:       "uplink",
// 		SslProfile: types.InterRouterProfile,
// 		Host:       types.TransportServiceName + "." + namespace + ".svc.cluster.local",
// 		Port:       strconv.Itoa(int(types.EdgeListenerPort)),
// 		Role:       RoleEdge,
// 	})
// 	config.AddListener(Listener{
// 		Name: "amqp",
// 		Host: "localhost",
// 		Port: 5672,
// 	})
// 	svcPorts := definition.Ports
// 	ports := map[int]int{}
// 	if len(definition.Targets) > 0 {
// 		ports = definition.Targets[0].TargetPorts
// 	} else {
// 		for _, sp := range svcPorts {
// 			ports[sp] = sp
// 		}
// 	}
// 	for iPort, ePort := range ports {
// 		address := fmt.Sprintf("%s-%s:%d", definition.Address, "${POD_ID}", iPort)
// 		if definition.IsOfLocalOrigin() {
// 			name := fmt.Sprintf("egress:%d", ePort)
// 			host := definition.Headless.Name + "-${POD_ID}." + definition.Address + "." + namespace
// 			// in the originating site, just have egress bindings
// 			switch definition.Protocol {
// 			case "tcp":
// 				config.AddTCPConnector(TCPEndpoint{
// 					Name:    name,
// 					Host:    host,
// 					Port:    strconv.Itoa(ePort),
// 					Address: address,
// 					SiteID:  siteID,
// 				})
// 			default:
// 			}
// 		} else {
// 			name := fmt.Sprintf("ingress:%d", ePort)
// 			// in all other sites, just have ingress bindings
// 			switch definition.Protocol {
// 			case "tcp":
// 				config.AddTCPListener(TCPEndpoint{
// 					Name:    name,
// 					Port:    strconv.Itoa(iPort),
// 					Address: address,
// 					SiteID:  siteID,
// 				})
// 			default:
// 			}
// 		}
// 	}
// 	return MarshalRouterConfig(config)
// }

func disableMutualTLS(l *Listener) {
	l.SaslMechanisms = ""
	l.AuthenticatePeer = false
}

func InteriorListener(options types.RouterOptions) Listener {
	l := Listener{
		Name:             "interior-listener",
		Role:             RoleInterRouter,
		Port:             types.InterRouterListenerPort,
		SslProfile:       types.InterRouterProfile, // The skupper-internal profile needs to be filtered by the config-sync sidecar, in order to avoid deleting automesh connectors
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
		MaxFrameSize:     options.MaxFrameSize,
		MaxSessionFrames: options.MaxSessionFrames,
	}
	if options.DisableMutualTLS {
		disableMutualTLS(&l)
	}
	return l
}

func EdgeListener(options types.RouterOptions) Listener {
	l := Listener{
		Name:             "edge-listener",
		Role:             RoleEdge,
		Port:             types.EdgeListenerPort,
		SslProfile:       types.InterRouterProfile,
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
		MaxFrameSize:     options.MaxFrameSize,
		MaxSessionFrames: options.MaxSessionFrames,
	}
	if options.DisableMutualTLS {
		disableMutualTLS(&l)
	}
	return l
}

func GetInterRouterOrEdgeConnection(host string, connections []Connection) *Connection {
	for _, c := range connections {
		if (c.Role == "inter-router" || c.Role == "edge") && c.Host == host {
			return &c
		}
	}
	return nil
}

type ConfigUpdate interface {
	Apply(config *RouterConfig) bool
}
