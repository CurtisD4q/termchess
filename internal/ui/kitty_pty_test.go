package ui

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/creack/pty"
)

// TestDebugDumpOnKittyPTY runs the real binary on a pty whose environment is
// a Kitty terminal, including KITTY_PID. A Kitty-shaped reply is written if
// the process queries the tty. The frame must still be the block pieces, and
// the header must not name the kitty protocol, unless --graphics kitty was
// passed. --graphics off forces text even when config.json says kitty.
func TestDebugDumpOnKittyPTY(t *testing.T) {
	bin := buildTermchess(t)
	for _, tc := range []struct {
		name   string
		args   []string
		config string
		kitty  bool
	}{
		{name: "no flag", args: []string{"--debug-dump"}},
		{name: "saved kitty", args: []string{"--debug-dump"}, config: "kitty"},
		{name: "graphics off", args: []string{"--debug-dump", "--graphics", "off"}, config: "kitty"},
		{name: "explicit kitty", args: []string{"--debug-dump", "--graphics", "kitty"}, kitty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := runDebugDump(t, bin, tc.args, tc.config)
			plain := stripSGR(stripTermReplies(out))
			header, _, _ := strings.Cut(plain, "\n")
			if strings.Contains(strings.ToLower(header), "kitty") != tc.kitty {
				t.Fatalf("header kitty=%v, want %v:\n%s", strings.Contains(strings.ToLower(header), "kitty"), tc.kitty, header)
			}
			if !strings.Contains(plain, "Classic wood") || !strings.Contains(plain, "8×4") {
				t.Fatalf("header missing:\n%s", header)
			}
			if !strings.Contains(plain, "█") {
				t.Fatalf("no block pieces:\n%s", plain)
			}
			if !strings.Contains(out, "38;2;255;255;255") || !strings.Contains(out, "38;2;0;0;0") {
				t.Fatal("piece foreground colours are missing")
			}
			if strings.Contains(out, "\U0010EEEE") {
				t.Fatal("squares were replaced with kitty placeholders")
			}
			if !tc.kitty && strings.Contains(out, "\x1b_G") {
				t.Fatal("graphics stayed on without an explicit --graphics kitty")
			}
		})
	}
}

func buildTermchess(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "termchess")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/termchess")
	cmd.Dir = filepath.Join("..", "..")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build: %v\n%s", err, stderr.String())
	}
	return bin
}

func runDebugDump(t *testing.T, bin string, args []string, graphics string) string {
	t.Helper()
	home := t.TempDir()
	cfgDir := filepath.Join(home, "config")
	dataDir := filepath.Join(home, "data")
	if graphics != "" {
		dir := filepath.Join(cfgDir, "termchess")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := []byte("{\n  \"theme\": \"wood\",\n  \"graphics\": \"" + graphics + "\"\n}\n")
		if err := os.WriteFile(filepath.Join(dir, "config.json"), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer ptmx.Close()
	if _, err := term.MakeRaw(tty.Fd()); err != nil {
		tty.Close()
		t.Fatal(err)
	}
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 64, Cols: 160}); err != nil {
		tty.Close()
		t.Fatal(err)
	}

	cmd := exec.Command(bin, args...)
	cmd.Env = []string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + cfgDir,
		"XDG_DATA_HOME=" + dataDir,
		"PATH=" + os.Getenv("PATH"),
		"TERM=xterm-kitty",
		"COLORTERM=truecolor",
		"KITTY_WINDOW_ID=1",
		"KITTY_PID=4242",
		"KITTY_PUBLIC_KEY=0123456789abcdef",
		"TERMINFO=" + filepath.Join(home, "terminfo"),
		"LANG=C.UTF-8",
	}
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		tty.Close()
		t.Fatal(err)
	}
	tty.Close()
	defer func() { _ = cmd.Process.Kill() }()

	var out bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 8192)
		var acc []byte
		replied := map[string]bool{}
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				out.Write(chunk)
				acc = append(acc, chunk...)
				replyKittyQueries(ptmx, acc, replied)
				if len(acc) > 8192 {
					acc = append([]byte(nil), acc[len(acc)-2048:]...)
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) {
					readDone <- nil
					return
				}
				readDone <- err
				return
			}
		}
	}()

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-waitDone:
	case <-time.After(8 * time.Second):
		t.Fatal("debug-dump did not exit")
	}
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out reading the pty")
	}
	if waitErr != nil {
		t.Fatalf("%v\n%s", waitErr, out.String())
	}
	return out.String()
}

// stripTermReplies drops terminal replies and image sequences so the header
// check sees the text the program drew. A DCS version reply that names kitty
// is not the header.
func stripTermReplies(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != 0x1b || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch s[i+1] {
		case ']', 'P', '_', 'X', '^':
			i += 2
			for i < len(s) {
				if s[i] == 0x07 {
					i++
					break
				}
				if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
					i += 2
					break
				}
				i++
			}
		default:
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}

// replyKittyQueries answers the probes a real Kitty tty would answer. The
// version reply names kitty. Echo is off, so this reaches the program only
// if it reads stdin. It must not change the rendered frame.
func replyKittyQueries(ptmx *os.File, acc []byte, replied map[string]bool) {
	s := string(acc)
	write := func(key, reply string) {
		if replied[key] || !strings.Contains(s, key) {
			return
		}
		replied[key] = true
		_, _ = ptmx.Write([]byte(reply))
	}
	write("\x1b[>q", "\x1bP>|kitty(0.40.1)\x1b\\")
	write("\x1b_Ga=q", "\x1b_Gi=31;OK\x1b\\")
	write("\x1b[c", "\x1b[?62;c")
	write("\x1b]11;?", "\x1b]11;rgb:0000/0000/0000\x1b\\")
}
