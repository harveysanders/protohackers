package pestcontrol

import (
	"bufio"
	"fmt"
	"net"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type Client struct {
	conn net.Conn
	bufW *bufio.Writer
	bufR *bufio.Reader
}

func (c *Client) Connect(serverAddr string) error {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return err
	}

	c.conn = conn
	c.bufW = bufio.NewWriter(conn)
	c.bufR = bufio.NewReader(conn)
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// sendHello sends a Hello message to the Authority server.
func (c *Client) sendHello() error {
	if c.conn == nil {
		return fmt.Errorf("client not connected")
	}
	helloMsg := proto.MsgHello{
		Protocol: "pestcontrol",
		Version:  1,
	}
	msg, err := helloMsg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("hello.MarshalBinary: %w", err)
	}
	if _, err := c.bufW.Write(msg); err != nil {
		return fmt.Errorf("c.bufW.Write: %w", err)
	}
	return nil
}

func (c *Client) sendError(err error) error {
	if c.conn == nil {
		return fmt.Errorf("client not connected")
	}
	errMsg := proto.MsgError{Message: err.Error()}
	msg, err := errMsg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("hello.MarshalBinary: %w", err)
	}
	if _, err := c.bufW.Write(msg); err != nil {
		return fmt.Errorf("c.bufW.Write: %w", err)
	}
	return nil

}

// readMessage reads a single message from the client underlying connection.
// If the message checksum is invalid, a proto.ErrBadChecksum is returned.
func (c *Client) readMessage() (proto.Message, error) {
	var msg proto.Message
	if _, err := msg.ReadFrom(c.conn); err != nil {
		return msg, err
	}
	return msg, nil
}
