package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	acgv1 "github.com/p-/ai-credential-gateway/gen/acg/v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:4181", "gRPC server address")
	proxyKey := flag.String("proxy-key", "", "filter by proxy key")
	pathPrefix := flag.String("path-prefix", "", "filter by path prefix")
	flag.Parse()

	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := acgv1.NewRequestStreamServiceClient(conn)
	filter := &acgv1.StreamFilter{
		ProxyKey:   *proxyKey,
		PathPrefix: *pathPrefix,
	}

	p := tea.NewProgram(newModel(client, filter), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("245")).
			Padding(0, 1)

	statusGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	statusRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	methodStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	keyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
	pathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ipStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	bodyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type eventMsg *acgv1.RequestEvent
type errMsg error

type model struct {
	client   acgv1.RequestStreamServiceClient
	filter   *acgv1.StreamFilter
	viewport viewport.Model
	lines    []string
	ready    bool
	err      error
	width    int
	height   int
}

func newModel(client acgv1.RequestStreamServiceClient, filter *acgv1.StreamFilter) model {
	return model{
		client: client,
		filter: filter,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.startStream(), tea.WindowSize())
}

func (m model) startStream() tea.Cmd {
	return func() tea.Msg {
		stream, err := m.client.StreamRequests(context.Background(), m.filter)
		if err != nil {
			return errMsg(err)
		}
		ev, err := stream.Recv()
		if err != nil {
			return errMsg(err)
		}
		return eventMsg(ev)
	}
}

func (m model) listenStream() tea.Cmd {
	return m.startStream()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 3
		footerHeight := 1
		verticalMargins := headerHeight + footerHeight
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargins)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

	case eventMsg:
		line := formatEvent(msg)
		m.lines = append(m.lines, line)
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		m.viewport.GotoBottom()
		cmds = append(cmds, m.listenStream())

	case errMsg:
		m.err = msg
		return m, nil
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}
	if !m.ready {
		return "\n  Connecting...\n"
	}

	title := titleStyle.Render("⚡ AI Credential Gateway Live Stream")
	filterInfo := ""
	if m.filter.ProxyKey != "" || m.filter.PathPrefix != "" {
		parts := []string{}
		if m.filter.ProxyKey != "" {
			parts = append(parts, "key="+m.filter.ProxyKey)
		}
		if m.filter.PathPrefix != "" {
			parts = append(parts, "path="+m.filter.PathPrefix)
		}
		filterInfo = headerStyle.Render(" [" + strings.Join(parts, ", ") + "]")
	}
	header := title + filterInfo + "\n" + headerStyle.Render(
		fmt.Sprintf("%-19s  %-6s  %-10s  %-6s  %-40s  %-20s",
			"TIMESTAMP", "METHOD", "KEY", "STATUS", "PATH", "CLIENT"),
	) + "\n"

	footer := helpStyle.Render(" q: quit • ↑/↓: scroll")

	return header + m.viewport.View() + "\n" + footer
}

func formatEvent(ev *acgv1.RequestEvent) string {
	ts := ev.Timestamp.AsTime().Local().Format("2006-01-02 15:04:05")
	status := formatStatus(int(ev.StatusCode))
	method := methodStyle.Render(fmt.Sprintf("%-6s", ev.Method))
	key := keyStyle.Render(fmt.Sprintf("%-10s", ev.ProxyKey))
	path := pathStyle.Render(fmt.Sprintf("%-40s", truncate(ev.Path, 40)))
	ip := ipStyle.Render(fmt.Sprintf("%-20s", ev.ClientIp))

	line := fmt.Sprintf(" %-19s  %s  %s  %s  %s  %s", ts, method, key, status, path, ip)

	// Append truncated bodies if present.
	if len(ev.RequestBody) > 0 {
		line += "\n" + bodyStyle.Render("  → "+truncate(string(ev.RequestBody), 120))
	}
	if len(ev.ResponseBody) > 0 {
		line += "\n" + bodyStyle.Render("  ← "+truncate(string(ev.ResponseBody), 120))
	}

	return line
}

func formatStatus(code int) string {
	s := fmt.Sprintf("%-6d", code)
	switch {
	case code >= 200 && code < 300:
		return statusGreen.Render(s)
	case code >= 300 && code < 400:
		return statusYellow.Render(s)
	default:
		return statusRed.Render(s)
	}
}

func truncate(s string, max int) string {
	// Replace newlines for single-line display.
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
