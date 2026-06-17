package qdr

import (
	"context"
	"crypto/tls"

	amqp "github.com/interconnectedcloud/go-amqp"

	"github.com/eclipse-iofog/router/internal/messaging"
)

type TLSConfigRetriever interface {
	GetTlsConfig() (*tls.Config, error)
}

type ConnectionFactory struct {
	url    string
	config TLSConfigRetriever
}

func (f *ConnectionFactory) Connect() (messaging.Connection, error) {
	if f.config == nil {
		return dial(f.url, amqp.ConnMaxFrameSize(4294967295))
	}
	tlsConfig, err := f.config.GetTlsConfig()
	if err != nil {
		return nil, err
	}
	return dial(f.url, amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnTLSConfig(tlsConfig))
}

func dial(addr string, opts ...amqp.ConnOption) (*AMQPConnection, error) {
	client, err := amqp.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return &AMQPConnection{client: client, session: session}, nil
}

func (f *ConnectionFactory) URL() string {
	return f.url
}

func NewConnectionFactory(url string, config TLSConfigRetriever) *ConnectionFactory {
	return &ConnectionFactory{
		url:    url,
		config: config,
	}
}

type AMQPConnection struct {
	client  *amqp.Client
	session *amqp.Session
}

type AMQPSender struct {
	connection *AMQPConnection
	sender     *amqp.Sender
}

type AMQPReceiver struct {
	connection *AMQPConnection
	receiver   *amqp.Receiver
}

func (c *AMQPConnection) Close() {
	_ = c.client.Close()
}

func (c *AMQPConnection) Sender(address string) (messaging.Sender, error) {
	sender, err := c.session.NewSender(amqp.LinkTargetAddress(address))
	if err != nil {
		return nil, err
	}
	return &AMQPSender{connection: c, sender: sender}, nil
}

func (c *AMQPConnection) Receiver(address string, credit uint32) (messaging.Receiver, error) {
	receiver, err := c.session.NewReceiver(
		amqp.LinkSourceAddress(address),
		amqp.LinkCredit(credit),
	)
	if err != nil {
		return nil, err
	}
	return &AMQPReceiver{connection: c, receiver: receiver}, nil
}

func (s *AMQPSender) Send(msg *amqp.Message) error {
	return s.sender.Send(context.Background(), msg)
}

func (s *AMQPSender) Close() error {
	return s.sender.Close(context.Background())
}

func (s *AMQPReceiver) Receive() (*amqp.Message, error) {
	return s.receiver.Receive(context.Background())
}

func (s *AMQPReceiver) Accept(msg *amqp.Message) error {
	return msg.Accept()
}

func (s *AMQPReceiver) Close() error {
	return s.receiver.Close(context.Background())
}
