package dialog

import (
	"fmt"
	"github.com/charmbracelet/bubbles/v2/key"
	"github.com/charmbracelet/bubbles/v2/viewport"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode/internal/styles"
	"github.com/sst/opencode/internal/theme"
	"github.com/sst/opencode/internal/util"
	"strings"
)

type PermissionAction string

// Permission responses
const (
	PermissionAllow          PermissionAction = "once"
	PermissionAllowDirectory PermissionAction = "always_directory"
	PermissionAllowSession   PermissionAction = "always_session"
	PermissionDeny           PermissionAction = "reject"
)

// PermissionResponseMsg represents the user's response to a permission request
type PermissionResponseMsg struct {
	// Permission permission.PermissionRequest
	Action PermissionAction
}

// PermissionDialogComponent interface for permission dialog component
type PermissionDialogComponent interface {
	tea.Model
	tea.ViewModel
	SetPermission(permission *opencode.EventListResponseEventPermissionUpdated) tea.Cmd
}

type permissionsMapping struct {
	Left         key.Binding
	Right        key.Binding
	EnterSpace   key.Binding
	Allow        key.Binding
	AllowSession key.Binding
	Deny         key.Binding
	Tab          key.Binding
	Escape       key.Binding
}

var permissionsKeys = permissionsMapping{
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "switch options"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "switch options"),
	),
	EnterSpace: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	),
	Allow: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "allow"),
	),
	AllowSession: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "allow for session"),
	),
	Deny: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "deny"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel"),
	),
}

// permissionDialogComponent is the implementation of PermissionDialog
type permissionDialogComponent struct {
	width           int
	height          int
	permission      *opencode.EventListResponseEventPermissionUpdated
	windowSize      tea.WindowSizeMsg
	contentViewPort viewport.Model
	selectedOption  int // 0: Allow, 1: Allow for session, 2: Deny

	diffCache     map[string]string
	markdownCache map[string]string
}

func (p *permissionDialogComponent) Init() tea.Cmd {
	return p.contentViewPort.Init()
}

func (p *permissionDialogComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.windowSize = msg
		cmd := p.SetSize()
		cmds = append(cmds, cmd)
		p.markdownCache = make(map[string]string)
		p.diffCache = make(map[string]string)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, permissionsKeys.Right) || key.Matches(msg, permissionsKeys.Tab):
			p.selectedOption = (p.selectedOption + 1) % 3
			return p, nil
		case key.Matches(msg, permissionsKeys.Left):
			p.selectedOption = (p.selectedOption + 2) % 3
		case key.Matches(msg, permissionsKeys.EnterSpace):
			return p, p.selectCurrentOption()
		case key.Matches(msg, permissionsKeys.Escape):
			return p, util.CmdHandler(PermissionResponseMsg{Action: PermissionDeny})
		default:
			// Pass other keys to viewport
			viewPort, cmd := p.contentViewPort.Update(msg)
			p.contentViewPort = viewPort
			cmds = append(cmds, cmd)
		}
	}

	return p, tea.Batch(cmds...)
}

func (p *permissionDialogComponent) selectCurrentOption() tea.Cmd {
	var action PermissionAction

	switch p.selectedOption {
	case 0:
		action = PermissionAllow
	case 1:
		action = PermissionAllowDirectory
	case 2:
		action = PermissionDeny
	}

	return util.CmdHandler(PermissionResponseMsg{Action: action}) // , Permission: p.permission})
}

func (p *permissionDialogComponent) renderButtons() string {
	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle().Foreground(t.Text())

	allowStyle := baseStyle
	allowSessionStyle := baseStyle
	denyStyle := baseStyle
	spacerStyle := baseStyle.Background(t.Background())

	// Style the selected button
	switch p.selectedOption {
	case 0:
		allowStyle = allowStyle.Background(t.Primary()).Foreground(t.Background())
		allowSessionStyle = allowSessionStyle.Background(t.Background()).Foreground(t.Primary())
		denyStyle = denyStyle.Background(t.Background()).Foreground(t.Primary())
	case 1:
		allowStyle = allowStyle.Background(t.Background()).Foreground(t.Primary())
		allowSessionStyle = allowSessionStyle.Background(t.Primary()).Foreground(t.Background())
		denyStyle = denyStyle.Background(t.Background()).Foreground(t.Primary())
	case 2:
		allowStyle = allowStyle.Background(t.Background()).Foreground(t.Primary())
		allowSessionStyle = allowSessionStyle.Background(t.Background()).Foreground(t.Primary())
		denyStyle = denyStyle.Background(t.Primary()).Foreground(t.Background())
	}

	allowButton := allowStyle.Padding(0, 1).Render("Yes")
	allowSessionButton := allowSessionStyle.Padding(0, 1).Render("Yes, don't ask again in this directory")
	denyButton := denyStyle.Padding(0, 1).Render("No, tell OpenCode what to do differently")

	content := lipgloss.JoinHorizontal(
		lipgloss.Left,
		allowButton,
		spacerStyle.Render("  "),
		allowSessionButton,
		spacerStyle.Render("  "),
		denyButton,
		spacerStyle.Render("  "),
	)

	remainingWidth := p.width - lipgloss.Width(content)
	if remainingWidth > 0 {
		content = spacerStyle.Render(strings.Repeat(" ", remainingWidth)) + content
	}
	return content
}

func (p *permissionDialogComponent) renderHeader() string {
	if p.permission == nil {
		return ""
	}
	
	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle()

	toolKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("Tool")
	toolValue := baseStyle.
		Foreground(t.Text()).
		Width(p.width - lipgloss.Width(toolKey)).
		Render(fmt.Sprintf(": %s", p.permission.Properties.ID))

	// Get path from metadata if available
	pathKey := baseStyle.Foreground(t.TextMuted()).Bold(true).Render("Path")
	pathValue := ""
	if path, ok := p.permission.Properties.Metadata["path"].(string); ok {
		pathValue = baseStyle.
			Foreground(t.Text()).
			Width(p.width - lipgloss.Width(pathKey)).
			Render(fmt.Sprintf(": %s", path))
	}

	headerParts := []string{
		lipgloss.JoinHorizontal(
			lipgloss.Left,
			toolKey,
			toolValue,
		),
		baseStyle.Render(strings.Repeat(" ", p.width)),
	}

	if pathValue != "" {
		headerParts = append(headerParts,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				pathKey,
				pathValue,
			),
			baseStyle.Render(strings.Repeat(" ", p.width)),
		)
	}

	// Add tool-specific header information
	switch p.permission.Properties.ID {
	case "bash":
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("Command"))
	case "edit":
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("Diff"))
	case "write":
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("Diff"))
	case "fetch":
		headerParts = append(headerParts, baseStyle.Foreground(t.TextMuted()).Width(p.width).Bold(true).Render("URL"))
	}

	return lipgloss.NewStyle().Background(t.Background()).Render(lipgloss.JoinVertical(lipgloss.Left, headerParts...))
}

func (p *permissionDialogComponent) renderBashContent() string {
	if p.permission == nil {
		return ""
	}
	
	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle()

	if cmd, ok := p.permission.Properties.Metadata["command"].(string); ok {
		content := fmt.Sprintf("```bash\n%s\n```", cmd)

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.Properties.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(p.width - 10, t.Background())
			return r.Render(content)
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogComponent) renderEditContent() string {
	if p.permission == nil {
		return ""
	}
	
	if diffData, ok := p.permission.Properties.Metadata["diff"].(string); ok {
		diff := p.GetOrSetDiff(p.permission.Properties.ID, func() (string, error) {
			return diffData, nil
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogComponent) renderPatchContent() string {
	if p.permission == nil {
		return ""
	}
	
	if diffData, ok := p.permission.Properties.Metadata["diff"].(string); ok {
		diff := p.GetOrSetDiff(p.permission.Properties.ID, func() (string, error) {
			return diffData, nil
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogComponent) renderWriteContent() string {
	if p.permission == nil {
		return ""
	}
	
	if diffData, ok := p.permission.Properties.Metadata["diff"].(string); ok {
		diff := p.GetOrSetDiff(p.permission.Properties.ID, func() (string, error) {
			return diffData, nil
		})

		p.contentViewPort.SetContent(diff)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogComponent) renderFetchContent() string {
	if p.permission == nil {
		return ""
	}
	
	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle()

	if url, ok := p.permission.Properties.Metadata["url"].(string); ok {
		content := fmt.Sprintf("```\n%s\n```", url)

		// Use the cache for markdown rendering
		renderedContent := p.GetOrSetMarkdown(p.permission.Properties.ID, func() (string, error) {
			r := styles.GetMarkdownRenderer(p.width - 10, t.Background())
			return r.Render(content)
		})

		finalContent := baseStyle.
			Width(p.contentViewPort.Width()).
			Render(renderedContent)
		p.contentViewPort.SetContent(finalContent)
		return p.styleViewport()
	}
	return ""
}

func (p *permissionDialogComponent) renderDefaultContent() string {
	if p.permission == nil {
		return ""
	}
	
	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle()

	content := p.permission.Properties.Title

	// Use the cache for markdown rendering
	renderedContent := p.GetOrSetMarkdown(p.permission.Properties.ID, func() (string, error) {
		r := styles.GetMarkdownRenderer(p.width - 10, t.Background())
		return r.Render(content)
	})

	finalContent := baseStyle.
		Width(p.contentViewPort.Width()).
		Render(renderedContent)
	p.contentViewPort.SetContent(finalContent)

	if renderedContent == "" {
		return ""
	}

	return p.styleViewport()
}

func (p *permissionDialogComponent) styleViewport() string {
	t := theme.CurrentTheme()
	contentStyle := styles.NewStyle().Background(t.Background())

	return contentStyle.Render(p.contentViewPort.View())
}

func (p *permissionDialogComponent) render() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}

	t := theme.CurrentTheme()
	baseStyle := styles.NewStyle()

	title := baseStyle.
		Bold(true).
		Width(p.width - 4).
		Foreground(t.Primary()).
		Render("Permission Required")
	
	// Render header
	headerContent := p.renderHeader()
	
	// Render buttons
	buttons := p.renderButtons()

	// Calculate content height dynamically based on window size
	p.contentViewPort.SetHeight(p.height - lipgloss.Height(headerContent) - lipgloss.Height(buttons) - 2 - lipgloss.Height(title))
	p.contentViewPort.SetWidth(p.width - 4)

	// Render content based on tool type
	var contentFinal string
	if p.permission != nil {
		switch p.permission.Properties.ID {
		case "bash":
			contentFinal = p.renderBashContent()
		case "edit":
			contentFinal = p.renderEditContent()
		case "patch":
			contentFinal = p.renderPatchContent()
		case "write":
			contentFinal = p.renderWriteContent()
		case "fetch":
			contentFinal = p.renderFetchContent()
		default:
			contentFinal = p.renderDefaultContent()
		}
	}

	content := lipgloss.JoinVertical(
		lipgloss.Top,
		title,
		baseStyle.Render(strings.Repeat(" ", lipgloss.Width(title))),
		headerContent,
		contentFinal,
		buttons,
		baseStyle.Render(strings.Repeat(" ", p.width-4)),
	)

	return baseStyle.
		Padding(1, 0, 0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderBackground(t.Background()).
		BorderForeground(t.TextMuted()).
		Width(p.width).
		Height(p.height).
		Render(content)
}


func (p *permissionDialogComponent) View() string {
	return p.render()
}

func (p *permissionDialogComponent) SetSize() tea.Cmd {
	if p.windowSize.Width == 0 || p.windowSize.Height == 0 {
		return nil
	}

	// Default dialog size
	p.width = int(float64(p.windowSize.Width) * 0.6)
	p.height = int(float64(p.windowSize.Height) * 0.3)

	// Ensure minimum size
	if p.width < 50 {
		p.width = 50
	}
	if p.height < 10 {
		p.height = 10
	}

	return nil
}

func (p *permissionDialogComponent) SetPermission(permission *opencode.EventListResponseEventPermissionUpdated) tea.Cmd {
	p.permission = permission
	return p.SetSize()
}

// Helper to get or set cached diff content
func (c *permissionDialogComponent) GetOrSetDiff(key string, generator func() (string, error)) string {
	if cached, ok := c.diffCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error formatting diff: %v", err)
	}

	c.diffCache[key] = content

	return content
}

// Helper to get or set cached markdown content
func (c *permissionDialogComponent) GetOrSetMarkdown(key string, generator func() (string, error)) string {
	if cached, ok := c.markdownCache[key]; ok {
		return cached
	}

	content, err := generator()
	if err != nil {
		return fmt.Sprintf("Error rendering markdown: %v", err)
	}

	c.markdownCache[key] = content

	return content
}

func NewPermissionDialogCmp() PermissionDialogComponent {
	// Create viewport for content
	contentViewport := viewport.New()

	return &permissionDialogComponent{
		contentViewPort: contentViewport,
		selectedOption:  0, // Default to "Allow"
		diffCache:       make(map[string]string),
		markdownCache:   make(map[string]string),
	}
}
