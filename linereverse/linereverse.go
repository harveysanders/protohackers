package linereverse

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/harveysanders/protohackers/linereverse/lrcp"
)

type App struct {
	in  chan lrcp.SessionMsg
	out chan lrcp.SessionMsg
}

func New() *App {
	return &App{
		in:  make(chan lrcp.SessionMsg, 1024),
		out: make(chan lrcp.SessionMsg, 1024),
	}
}

func (a *App) Run(ctx context.Context, address string) error {
	l, err := lrcp.Listen(address, a.in, a.out)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				l.Close()
				return
			case msg := <-a.in:
				line := msg.Data
				reversed, err := reverseLine(line)
				if err != nil {
					log.Printf("reverseLine: %v", err)
				}
				msg.Data = reversed
				a.out <- msg
			}
		}
	}()

	l.Accept()
	return nil
}

func reverseLine(line []byte) ([]byte, error) {
	var out bytes.Buffer
	for i := len(line) - 1; i >= 0; i-- {
		err := out.WriteByte(line[i])
		if err != nil {
			return out.Bytes(), err
		}
	}
	if _, err := out.WriteRune('\n'); err != nil {
		return out.Bytes(), err
	}
	return out.Bytes(), nil
}
