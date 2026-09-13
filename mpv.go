package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

type MPV struct {
	cmd  *exec.Cmd
	conn net.Conn
	mu   sync.Mutex
	pipe string
}

type mpvMessage struct {
	Event  string          `json:"event"`
	ID     int             `json:"id"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error"`
	Reason string          `json:"reason"`
}

func NewMPV() (*MPV, error) {
	pipe := `\\.\pipe\mpvx-` + fmt.Sprint(os.Getpid())
	return &MPV{pipe: pipe}, nil
}

func (m *MPV) Start() error {
	path, err := exec.LookPath("mpv")
	if err != nil {
		return errors.New("找不到 mpv，请确认 mpv.exe 已加入 PATH")
	}
	m.cmd = exec.Command(path,
		"--idle=yes", "--no-video", "--force-window=no",
		"--terminal=no", "--really-quiet", "--loop-file=no", "--input-ipc-server="+m.pipe,
	)
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("启动 mpv 失败: %w", err)
	}

	var dialErr error
	timeout := 200 * time.Millisecond
	for i := 0; i < 30; i++ {
		m.conn, dialErr = winio.DialPipe(m.pipe, &timeout)
		if dialErr == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("连接 mpv IPC 失败: %w", dialErr)
}

func (m *MPV) ReadEvents(ch chan<- mpvMessage) {
	if m.conn == nil {
		return
	}
	go func() {
		defer close(ch)
		s := bufio.NewScanner(m.conn)
		s.Buffer(make([]byte, 4096), 1024*1024)
		for s.Scan() {
			var msg mpvMessage
			if json.Unmarshal(s.Bytes(), &msg) == nil {
				ch <- msg
			}
		}
	}()
}

func (m *MPV) Command(args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.conn == nil {
		return errors.New("mpv IPC 未连接")
	}
	payload, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_ = m.conn.SetWriteDeadline(time.Now().Add(time.Second))
	_, err = m.conn.Write(payload)
	return err
}

func (m *MPV) Load(path string, replace bool) error {
	mode := "append"
	if replace {
		mode = "replace"
	}
	return m.Command("loadfile", filepath.ToSlash(path), mode)
}

func (m *MPV) Observe() error {
	for id, prop := range []string{"time-pos", "duration", "pause", "media-title", "path", "volume"} {
		if err := m.Command("observe_property", id+1, prop); err != nil {
			return err
		}
	}
	return nil
}

func (m *MPV) Close() {
	if m.conn != nil {
		_ = m.Command("quit", 0)
		_ = m.conn.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
}
