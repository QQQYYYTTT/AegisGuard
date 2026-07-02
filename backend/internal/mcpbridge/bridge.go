package mcpbridge

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type Bridge struct {
	cfg      Config
	client   *BackendClient
	registry *SchemaRegistry
}

type Config struct {
	BackendURL string
	AgentID    string
	SessionID  string
	TaskID     string
	Command    []string
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
}

func New(cfg Config) (*Bridge, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("missing real MCP command")
	}
	if strings.TrimSpace(cfg.BackendURL) == "" {
		return nil, fmt.Errorf("missing backend url")
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	return &Bridge{
		cfg:      cfg,
		client:   NewBackendClient(strings.TrimRight(cfg.BackendURL, "/")),
		registry: NewSchemaRegistry(),
	}, nil
}

func (b *Bridge) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, b.cfg.Command[0], b.cfg.Command[1:]...)
	cmd.Stderr = b.cfg.Stderr

	childIn, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	childOut, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	defer func() {
		_ = childIn.Close()
		_ = childOut.Close()
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- b.forwardHostToChild(ctx, childIn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- b.forwardChildToHost(ctx, childOut)
	}()

	waitErr := make(chan error, 1)
	go func() {
		waitErr <- cmd.Wait()
	}()

	wg.Wait()
	_ = childIn.Close()

	select {
	case err := <-waitErr:
		if err != nil {
			return err
		}
	default:
	}

	select {
	case err := <-errCh:
		if err != nil && err != io.EOF {
			return err
		}
	default:
	}
	return <-waitErr
}

func (b *Bridge) forwardHostToChild(ctx context.Context, childIn io.Writer) error {
	reader := NewFrameReader(b.cfg.Stdin)
	writer := NewFrameWriter(childIn)
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			return err
		}
		out, err := b.handleHostFrame(ctx, frame)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			continue
		}
		if err := writer.WriteFrame(out); err != nil {
			return err
		}
	}
}

func (b *Bridge) forwardChildToHost(ctx context.Context, childOut io.Reader) error {
	reader := NewFrameReader(childOut)
	writer := NewFrameWriter(b.cfg.Stdout)
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			return err
		}
		out, err := b.handleChildFrame(ctx, frame)
		if err != nil {
			return err
		}
		if len(out) == 0 {
			continue
		}
		if err := writer.WriteFrame(out); err != nil {
			return err
		}
	}
}

func (b *Bridge) handleHostFrame(ctx context.Context, frame []byte) ([]byte, error) {
	msg := RPCMessage{}
	if err := json.Unmarshal(frame, &msg); err != nil {
		return frame, nil
	}
	if msg.Method == "" {
		return frame, nil
	}

	switch msg.Method {
	case "tools/list":
		b.registry.RememberPending(msg.IDKey(), "tools/list", "")
		return frame, nil
	case "tools/call":
		return b.handleToolsCall(ctx, frame, &msg)
	default:
		return frame, nil
	}
}

func (b *Bridge) handleChildFrame(ctx context.Context, frame []byte) ([]byte, error) {
	msg := RPCMessage{}
	if err := json.Unmarshal(frame, &msg); err != nil {
		return frame, nil
	}

	if msg.Result == nil {
		return frame, nil
	}

	if method, ok := b.registry.PopPendingMethod(msg.IDKey()); ok {
		switch method {
		case "tools/list":
			b.captureToolList(msg.Result)
			return frame, nil
		case "tools/call":
			return b.handleToolsCallResult(ctx, frame)
		default:
			return frame, nil
		}
	}

	return frame, nil
}

func (b *Bridge) handleToolsCall(ctx context.Context, frame []byte, msg *RPCMessage) ([]byte, error) {
	toolName, params := extractToolCall(msg.Params)
	if toolName == "" {
		return frame, nil
	}
	schema := b.registry.Schema(toolName)
	eval, err := b.client.EvaluateAction(ctx, BridgeActionRequest{
		RequestID: msg.IDKey(),
		ToolName:  toolName,
		AgentID:   b.cfg.AgentID,
		SessionID: b.cfg.SessionID,
		TaskID:    b.cfg.TaskID,
		Params:    params,
		Schema:    schema,
	})
	if err != nil {
		return nil, err
	}
	if eval.Data.Action.Decision.String() != "Allow" {
		return buildErrorResponse(msg, eval.Data.Action.Reason), nil
	}
	b.registry.RememberPending(msg.IDKey(), "tools/call", toolName)
	return frame, nil
}

func (b *Bridge) handleToolsCallResult(ctx context.Context, frame []byte) ([]byte, error) {
	msg := RPCMessage{}
	if err := json.Unmarshal(frame, &msg); err != nil {
		return frame, nil
	}
	toolName := b.registry.PopPendingTool(msg.IDKey())
	if toolName == "" {
		return frame, nil
	}

	eval, err := b.client.EvaluateReturn(ctx, BridgeReturnRequest{
		RequestID:    msg.IDKey(),
		ToolName:     toolName,
		AgentID:      b.cfg.AgentID,
		ResponseBody: string(frame),
	})
	if err != nil {
		return nil, err
	}
	if eval.Data.Return == nil {
		return frame, nil
	}
	switch eval.Data.Return.Decision.String() {
	case "Block", "Deny":
		return buildErrorResponse(&msg, eval.Data.Return.Reason), nil
	case "Degrade":
		if strings.TrimSpace(eval.Data.ResponseBody) != "" {
			return []byte(eval.Data.ResponseBody), nil
		}
	}
	return frame, nil
}

func (b *Bridge) captureToolList(result json.RawMessage) {
	tools := parseToolList(result)
	for name, schema := range tools {
		b.registry.Set(name, schema)
	}
}

func buildErrorResponse(msg *RPCMessage, reason string) []byte {
	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"error": map[string]any{
			"code":    -32001,
			"message": "request blocked by AegisGuard",
			"data": map[string]any{
				"reason": reason,
			},
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

type FrameReader struct {
	reader *bufio.Reader
}

func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{reader: bufio.NewReader(r)}
}

func (r *FrameReader) ReadFrame() ([]byte, error) {
	peek, err := r.reader.Peek(14)
	if err == nil && bytes.HasPrefix(bytes.ToLower(peek), []byte("content-length")) {
		return r.readContentLengthFrame()
	}
	line, err := r.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return r.ReadFrame()
	}
	return line, nil
}

func (r *FrameReader) readContentLengthFrame() ([]byte, error) {
	length := 0
	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			_, err := fmt.Sscanf(line, "Content-Length: %d", &length)
			if err != nil {
				_, err = fmt.Sscanf(line, "content-length: %d", &length)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if length <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r.reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

type FrameWriter struct {
	writer io.Writer
}

func NewFrameWriter(w io.Writer) *FrameWriter {
	return &FrameWriter{writer: w}
}

func (w *FrameWriter) WriteFrame(frame []byte) error {
	if len(frame) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(w.writer, "Content-Length: %d\r\n\r\n", len(frame))
	if err != nil {
		return err
	}
	_, err = w.writer.Write(frame)
	return err
}
