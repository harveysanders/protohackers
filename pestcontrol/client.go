package pestcontrol

import (
	"bufio"
	"context"
	"fmt"
	"net"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
)

type Client struct {
	conn net.Conn
	bufW *bufio.Writer
	bufR *bufio.Reader
	// siteID is the unique identifier for the Site Authority. It is nil if the client has not yet dialed and connected the Authority.
	siteID *uint32
}

// Connect establishes a connection to the Authority server at the provided address.
func (c *Client) Connect(ctx context.Context, serverAddr string) error {
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
	c.siteID = nil
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// dialAuthority dials a specific Authority server identified by siteID.
func (c *Client) dialAuthority(siteID uint32) error {
	if c.conn == nil {
		return fmt.Errorf("client not connected")
	}
	dialMsg := proto.MsgDialAuthority{Site: siteID}
	msg, err := dialMsg.MarshalBinary()
	if err != nil {
		return fmt.Errorf("dialMsg.MarshalBinary: %w", err)
	}
	if _, err := c.bufW.Write(msg); err != nil {
		return fmt.Errorf("c.bufW.Write: %w", err)
	}
	if err := c.bufW.Flush(); err != nil {
		return fmt.Errorf("c.bufW.Flush: %w", err)
	}
	c.siteID = &siteID
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

// establishSiteConnection establishes a connection to the Authority server at serverAddr and dials the Authority server identified by siteID. It returns the target populations message from the specified site.
func (c *Client) establishSiteConnection(ctx context.Context, serverAddr string, siteID uint32) (proto.MsgTargetPopulations, error) {
	if err := c.Connect(ctx, serverAddr); err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("client.Connect: %v", err)
	}
	if err := c.sendHello(); err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("client.sendHello: %v", err)
	}

	// Ensure we received a Hello message from the Authority server.
	msg, err := c.readMessage()
	if err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("client.readMessage: expected 'hello' resp: %v", err)
	}
	if _, err := msg.ToMsgHello(); err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("msg.ToMsgHello: %v", err)
	}

	if err := c.dialAuthority(siteID); err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("client.dialAuthority: %v", err)
	}

	// Read the expected target populations message from the Authority server.
	msg, err = c.readMessage()
	if err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("client.readMessage: expected pop. resp: %v", err)
	}

	popResp, err := msg.ToMsgTargetPopulations()
	if err != nil {
		return proto.MsgTargetPopulations{}, fmt.Errorf("msg.ToMsgTargetPopulations: %v", err)
	}

	return popResp, nil
}
