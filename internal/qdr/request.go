package qdr

import (
	"context"
	"fmt"

	amqp "github.com/interconnectedcloud/go-amqp"
)

const (
	VersionProperty string = "version"
)

type Request struct {
	Address    string
	Type       string
	Version    string
	Properties map[string]any
	Body       string
}

type Response struct {
	Type       string
	Version    string
	Properties map[string]any
	Body       string
}

type RequestResponse interface {
	Request(request *Request) (*Response, error)
}

type RequestServer struct {
	pool    *AgentPool
	address string
	handler RequestResponse
}

func NewRequestServer(address string, handler RequestResponse, pool *AgentPool) *RequestServer {
	return &RequestServer{
		pool, address, handler,
	}
}

func (s *RequestServer) Run(ctx context.Context) error {
	agent, err := s.pool.Get()
	if err != nil {
		return fmt.Errorf("could not get management agent: %w", err)
	}
	defer agent.Close()

	receiver, err := agent.newReceiver(s.address)
	if err != nil {
		return fmt.Errorf("could not open receiver for %s: %w", s.address, err)
	}
	for {
		err = s.serve(ctx, receiver, agent.anonymous)
		if err != nil {
			return fmt.Errorf("error handling request for %s: %w", s.address, err)
		}
	}
}

func (s *RequestServer) serve(ctx context.Context, receiver *amqp.Receiver, sender *amqp.Sender) error {
	for {
		requestMsg, err := receiver.Receive(ctx)
		if err != nil {
			return fmt.Errorf("failed reading request from %s: %s", s.address, err.Error())
		}

		request := Request{
			Address:    requestMsg.Properties.To,
			Type:       requestMsg.Properties.Subject,
			Properties: map[string]any{},
		}
		for k, v := range requestMsg.ApplicationProperties {
			if k == VersionProperty {
				if version, ok := v.(string); ok {
					request.Version = version
				}
			} else {
				request.Properties[k] = v
			}
		}
		if body, ok := requestMsg.Value.(string); ok {
			request.Body = body
		}

		response, err := s.handler.Request(&request)
		if err != nil {
			_ = requestMsg.Reject(&amqp.Error{
				Condition:   amqp.ErrorInternalError,
				Description: err.Error(),
			})
			return err
		}
		_ = requestMsg.Accept()
		responseMsg := amqp.Message{
			Properties: &amqp.MessageProperties{
				To:      requestMsg.Properties.ReplyTo,
				Subject: response.Type,
			},
			ApplicationProperties: map[string]any{},
			Value:                 response.Body,
		}
		correlationID, ok := AsUint64(requestMsg.Properties.CorrelationID)
		if !ok {
			responseMsg.Properties.CorrelationID = correlationID
		}
		for k, v := range response.Properties {
			responseMsg.ApplicationProperties[k] = v
		}
		responseMsg.ApplicationProperties[VersionProperty] = response.Version

		err = sender.Send(ctx, &responseMsg)
		if err != nil {
			_ = requestMsg.Reject(&amqp.Error{
				Condition:   amqp.ErrorInternalError,
				Description: "Could not send response: " + err.Error(),
			})
			return fmt.Errorf("could not send response: %w", err)
		}
	}
}
