package main

import (
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dcboy/go-asar/asar"
)

// --- 基础配置 ---
const (
	AppTitle          = "Devil Connection Mod 安装工具"
	AppVersion        = "v1.0.0"
	TargetExe         = "DevilConnection.exe"
	TargetApp         = "DevilConnection.app"
	SteamAppID        = "3054820"
	DLExpectedContent = "export function initSteam(){return null;}"
)

// --- 嵌入资源 ---
//go:embed ModLoader/app.asar ModLoader/app_dl.asar ModLoader/app_macos.asar
var embeddedAssets embed.FS

// --- UI 样式模板 ---
var (
	StyleTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7FF")).MarginBottom(1)
	StyleItem    = lipgloss.NewStyle().PaddingLeft(2)
	StyleSelect  = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#00FF87")).Bold(true)
	StyleStatus  = lipgloss.NewStyle().Bold(true).Padding(0, 1).MarginRight(1).Foreground(lipgloss.Color("#FFFFFF"))
	StyleSuccess = StyleStatus.Copy().Background(lipgloss.Color("#00AF00"))
	StyleError   = StyleStatus.Copy().Background(lipgloss.Color("#D70000"))
	StyleInfo    = StyleStatus.Copy().Background(lipgloss.Color("#005FDF"))
	StyleHelp    = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Italic(true)
)

// --- 核心业务逻辑模块 ---

type InstallStatus int

const (
	StatusNotInstalled InstallStatus = iota
	StatusInstalled
	StatusUnknown
)

type Installer struct {
	RootDir    string
	AsarPath   string
	BackupPath string
}

func NewInstaller(root string) *Installer {
	var asarRel, bakRel string
	if runtime.GOOS == "darwin" && strings.HasSuffix(root, ".app") {
		asarRel = "Contents/Resources/app.asar"
		bakRel = "Contents/Resources/plugins/app.bak.asar"
	} else {
		asarRel = "resources/app.asar"
		bakRel = "resources/plugins/app.bak.asar"
	}

	return &Installer{
		RootDir:    root,
		AsarPath:   filepath.Join(root, asarRel),
		BackupPath: filepath.Join(root, bakRel),
	}
}

// 检查错误是否由文件占用引起
func isLockedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Windows: "used by another process", "access is denied"
	// Unix: "text file busy"
	return strings.Contains(msg, "process cannot access") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "text file busy")
}

// CheckStatus 检查当前安装状态
func (i *Installer) CheckStatus() InstallStatus {
	if _, err := os.Stat(i.BackupPath); err == nil {
		return StatusInstalled
	}
	if _, err := os.Stat(i.AsarPath); err == nil {
		return StatusNotInstalled
	}
	return StatusUnknown
}

// DetectIsDL 检测是否为 DL 版
func (i *Installer) DetectIsDL() bool {
	path := i.AsarPath
	if _, err := os.Stat(i.BackupPath); err == nil {
		path = i.BackupPath
	}

	files, err := asar.ListPackage(path, false)
	if err != nil {
		return false
	}

	var steamJS string
	for _, f := range files {
		if strings.HasSuffix(f, "steam.js") {
			steamJS = f
			break
		}
	}
	if steamJS == "" {
		return false
	}

	content, err := asar.ExtractFile(path, steamJS, false)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(content)) == DLExpectedContent
}

// Install 执行安装操作
func (i *Installer) Install() (string, error) {
	currentStatus := i.CheckStatus()
	if currentStatus == StatusUnknown {
		return "", fmt.Errorf("找不到核心文件 app.asar")
	}

	// 1. 首次安装时备份原文件
	if currentStatus == StatusNotInstalled {
		if err := os.MkdirAll(filepath.Dir(i.BackupPath), 0755); err != nil {
			return "", err
		}
		// 尝试移动
		if err := os.Rename(i.AsarPath, i.BackupPath); err != nil {
			if isLockedError(err) {
				return "", fmt.Errorf("文件被占用, 请先关闭游戏后再安装.")
			}
			// 跨分区失败尝试拷贝
			if err := copyFile(i.AsarPath, i.BackupPath); err != nil {
				return "", fmt.Errorf("备份失败(可能被占用): %w", err)
			}
			// 拷贝成功后尝试删除原件，如果删不掉说明被占用
			if err := os.Remove(i.AsarPath); err != nil {
				_ = os.Remove(i.BackupPath) // 撤销备份
				return "", fmt.Errorf("无法修改游戏文件, 请检查游戏是否运行中.")
			}
		}
	}

	// 2. 选择资源
	isDL := i.DetectIsDL()
	patchSource := "ModLoader/app.asar"
	if runtime.GOOS == "darwin" {
		patchSource = "ModLoader/app_macos.asar"
	} else if isDL {
		patchSource = "ModLoader/app_dl.asar"
	}

	// 3. 释放补丁 (带占用检测)
	if err := extractEmbedded(patchSource, i.AsarPath); err != nil {
		if isLockedError(err) {
			return "", fmt.Errorf("补丁安装失败: 文件被占用, 请关闭游戏.")
		}
		return "", fmt.Errorf("补丁安装失败: %w", err)
	}

	if currentStatus == StatusInstalled {
		return "ModLoader 更新成功.", nil
	}
	return "ModLoader 安装成功.", nil
}

// Uninstall 卸载操作
func (i *Installer) Uninstall() error {
	if _, err := os.Stat(i.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("未发现备份文件, 无需还原.")
	}

	// 尝试重命名现有文件以检测占用
	tempMod := i.AsarPath + ".old_mod"
	if err := os.Rename(i.AsarPath, tempMod); err != nil {
		if isLockedError(err) {
			return fmt.Errorf("无法还原: 游戏文件被占用, 请先关闭游戏.")
		}
		return fmt.Errorf("操作被拒绝: %w", err)
	}

	// 还原备份
	err := os.Rename(i.BackupPath, i.AsarPath)
	if err != nil {
		if err2 := copyFile(i.BackupPath, i.AsarPath); err2 != nil {
			_ = os.Rename(tempMod, i.AsarPath) // 失败回滚
			return fmt.Errorf("还原失败: %w", err2)
		}
		_ = os.Remove(i.BackupPath)
	}

	// 清理
	_ = os.Remove(tempMod)
	_ = os.Remove(filepath.Dir(i.BackupPath))
	return nil
}

// --- TUI 模型 ---

type msgStatus struct {
	Level string
	Text  string
}

type model struct {
	installer *Installer
	spinner   spinner.Model
	choices   []string
	cursor    int
	status    msgStatus
	loading   bool
	quitting  bool
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return model{
		choices: []string{"安装/更新补丁", "卸载/还原原版"},
		spinner: s,
		status:  msgStatus{"INF", "正在初始化..."},
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.detectEnv)
}

func (m model) detectEnv() tea.Msg {
	exePath, _ := os.Executable()
	currDir := filepath.Dir(exePath)
	pathsToTry := []string{currDir, filepath.Dir(currDir)}

	for _, p := range pathsToTry {
		inst := NewInstaller(p)
		if inst.CheckStatus() != StatusUnknown {
			return msgStatus{"PATH", p}
		}
	}
	return msgStatus{"ERR", "未找到游戏. 请将工具放入游戏根目录. 确保当前目录下存在 DevilConnection.exe 或 DevilConnection.app"}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgStatus:
		m.loading = false
		if msg.Level == "PATH" {
			m.installer = NewInstaller(msg.Text)
			m.updateMenu()
			m.status = msgStatus{"OK", "环境已就绪."}
			return m, nil
		}
		m.status = msg
		// 每次操作状态更新后刷新菜单
		m.updateMenu()
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "w":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "s":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter":
			if m.installer == nil {
				return m, nil
			}
			m.loading = true
			return m, m.handleAction()
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) updateMenu() {
	if m.installer == nil {
		return
	}
	status := m.installer.CheckStatus()
	if status == StatusInstalled {
		m.choices[0] = "更新补丁"
		m.choices[1] = "卸载补丁 (还原游戏)"
	} else {
		m.choices[0] = "安装补丁"
		m.choices[1] = "卸载补丁 (无需操作)"
	}
}

func (m model) handleAction() tea.Cmd {
	return func() tea.Msg {
		var err error
		var msg string
		if m.cursor == 0 {
			msg, err = m.installer.Install()
			if err == nil {
				return msgStatus{"OK", msg}
			}
		} else {
			err = m.installer.Uninstall()
			if err == nil {
				return msgStatus{"OK", "还原成功."}
			}
		}
		return msgStatus{"ERR", err.Error()}
	}
}

func (m model) View() string {
	if m.quitting {
		return "\n  正在退出...\n"
	}
	var b strings.Builder
	b.WriteString(StyleTitle.Render(fmt.Sprintf("%s %s", AppTitle, AppVersion)) + "\n")

	if m.installer != nil {
		b.WriteString(fmt.Sprintf("  [路径] %s\n", m.installer.RootDir))
	} else {
		b.WriteString(StyleError.Render("  [错误] 未找到游戏!") + "\n")
	}
	b.WriteString("\n")

	for i, choice := range m.choices {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
			b.WriteString(StyleSelect.Render(cursor+choice) + "\n")
		} else {
			b.WriteString(StyleItem.Render(cursor+choice) + "\n")
		}
	}

	b.WriteString("\n" + strings.Repeat("-", 40) + "\n")
	if m.loading {
		b.WriteString(m.spinner.View() + " 处理中, 请勿关闭...")
	} else {
		tag := StyleInfo.Render(" 提示 ")
		if m.status.Level == "OK" {
			tag = StyleSuccess.Render(" 成功 ")
		}
		if m.status.Level == "ERR" {
			tag = StyleError.Render(" 失败 ")
		}
		b.WriteString(tag + " " + m.status.Text)
	}
	b.WriteString("\n" + StyleHelp.Render("  [↑/↓] 选择  [Enter] 确认  [Q] 退出"))
	return b.String()
}

// --- IO 辅助函数 ---

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

func extractEmbedded(srcName, dstPath string) error {
	src, err := embeddedAssets.Open(srcName)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}
}