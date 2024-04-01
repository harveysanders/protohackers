package main

import (
	"bufio"
	"fmt"
	"net"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type Client struct {
	// Authority server address
	serverAddr string
	conn       net.Conn
	bufW       *bufio.Writer
	bufR       *bufio.Reader
}

func NewClient(serverAddr string) *Client {
	return &Client{serverAddr: serverAddr}
}

func (c *Client) Connect() error {
	conn, err := net.Dial("tcp", c.serverAddr)
	if err != nil {
		return err
	}

	c.conn = conn
	c.bufR = bufio.NewReader(conn)
	c.bufW = bufio.NewWriter(conn)
	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// SendHello sends a Hello message to the Authority server.
func (c *Client) SendHello() error {
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
