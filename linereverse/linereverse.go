package linereverse

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/harveysanders/protohackers/linereverse/lrcp"
)

type App struct {
}

func New() *App {
	return &App{}
}

func (a *App) Run(port string) error {
	l, err := lrcp.Listen(port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}
		go func(c *lrcp.StableConn) {
			err := reverseLines(c, c)
			if err != nil {
				log.Printf("reverseLines: %v", err)
			}
		}(conn)
	}
}

// RevereLines reads lines from r, reverses the contents and writes the result to w.
func reverseLines(w io.Writer, r io.Reader) error {
	scr := bufio.NewScanner(r)
	scr.Split(bufio.ScanLines)

	for scr.Scan() {
		if scr.Err() != nil {
			return scr.Err()
		}
		line, err := reverseLine(scr.Bytes())
		if err != nil {
			return err
		}
		_, err = w.Write(append(line, '\n'))
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
