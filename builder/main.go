package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Cores para o terminal (ANSI)
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

func main() {
	fmt.Println(ColorCyan + "╔══════════════════════════════════════╗" + ColorReset)
	fmt.Println(ColorCyan + "║    FortressVision Native Builder     ║" + ColorReset)
	fmt.Println(ColorCyan + "╚══════════════════════════════════════╝" + ColorReset)

	start := time.Now()

	// 1. Configurar Ambiente
	setupEnvironment()

	// 2. Compilar Servidor
	if err := buildComponent("SERVIDOR (CGO + Static)", "servidor", "servidor/server.exe", true, "-extldflags=-static -s -w"); err != nil {
		fatal(err)
	}

	// 3. Compilar Cliente
	if err := buildComponent("CLIENTE (CGO + Static + GUI)", "cliente", "cliente/client.exe", true, "-extldflags=-static -s -w -H=windowsgui"); err != nil {
		fatal(err)
	}

	// 4. Compilar Launcher
	if err := buildComponent("LAUNCHER (Pure Go)", "launcher", "FortressVision.exe", false, "-s -w"); err != nil {
		fatal(err)
	}

	fmt.Printf("\n"+ColorCyan+"Build finalizada com sucesso em %v!"+ColorReset+"\n", time.Since(start).Round(time.Second))
	fmt.Println(ColorYellow + "Dica: Execute o 'FortressVision.exe' para jogar." + ColorReset)

	fmt.Println("\nPressione Enter para sair...")
	fmt.Scanln()
}

func setupEnvironment() {
	fmt.Println(ColorYellow + "\n[0/3] Configurando ambiente de compilação..." + ColorReset)

	// Adicionar MSYS2 ao PATH se estiver no Windows
	if runtime.GOOS == "windows" {
		msysPath := `C:\msys64\mingw64\bin`
		currentPath := os.Getenv("PATH")
		if !strings.Contains(currentPath, msysPath) {
			os.Setenv("PATH", msysPath+";"+currentPath)
			fmt.Printf("  - PATH atualizado: %s adicionado.\n", msysPath)
		}
		os.Setenv("CC", "gcc")
		fmt.Println("  - Compilador C: gcc (MSYS2)")
	}
}

func buildComponent(name, dir, output string, useCgo bool, ldflags string) error {
	fmt.Printf(ColorYellow+"\n[+] Compilando %s..."+ColorReset+"\n", name)

	cgoValue := "0"
	if useCgo {
		cgoValue = "1"
	}
	os.Setenv("CGO_ENABLED", cgoValue)

	args := []string{"build", "-ldflags", ldflags, "-o", output, "./" + dir}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("falha ao compilar %s: %v", name, err)
	}

	fmt.Printf(ColorGreen+"  - %s compilado com sucesso -> %s"+ColorReset+"\n", name, output)
	return nil
}

func fatal(err error) {
	fmt.Printf("\n"+ColorRed+"[ERRO FATAL] %v"+ColorReset+"\n", err)
	fmt.Println("Pressione Enter para sair...")
	fmt.Scanln()
	os.Exit(1)
}
