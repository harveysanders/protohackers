package linereverse

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/harveysanders/protohackers/linereverse/lrcp"
)

type App struct {
}

func New() *App {
	return &App{}
}

func (a *App) Run(ctx context.Context, address string) error {
	l, err := lrcp.Listen(address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return l.Close()
		default:
			conn, err := l.Accept()
			if err != nil {
				log.Printf("accept: %v", err)
			}
			go func(c *lrcp.StableConn) {
				err := reverseLines(c)
				if err != nil {
					log.Printf("reverseLines: %v", err)
				}
			}(conn)
		}
	}
}

// RevereLines reads lines from r, reverses the contents and writes the result to w.
func reverseLines(c *lrcp.StableConn) error {
	scr := bufio.NewScanner(c)
	scr.Split(bufio.ScanLines)

	for scr.Scan() {
		if scr.Err() != nil {
			return scr.Err()
		}
		line, err := reverseLine(scr.Bytes())
		if err != nil {
			return err
		}
		_, err = c.Write(append(line, '\n'))
		if err != nil {
			return err
		}
	}
	return nil
}

func reverseLine(line []byte) ([]byte, error) {
	var out bytes.Buffer
	for i := len(line); i >= 0; i-- {
		err := out.WriteByte(line[i])
		if err != nil {
			return out.Bytes(), err
		}
	}
	return out.Bytes(), nil
}
