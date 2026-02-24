package main

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║      FortressVision Launcher         ║")
	fmt.Println("╚══════════════════════════════════════╝")

	// 1. Iniciar o Servidor em uma nova janela (necessário para ver os logs)
	fmt.Println("[1/2] Iniciando Servidor...")
	serverCmd := exec.Command("cmd", "/c", "start", "FortressVision SERVER", "cmd", "/c", "cd servidor && .\\server.exe")
	if err := serverCmd.Run(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}

	// 2. Aguardar o servidor inicializar
	fmt.Println("Aguardando inicialização do servidor...")
	time.Sleep(2 * time.Second)

	// 3. Iniciar o Cliente silenciosamente (App GUI não precisa de CMD)
	fmt.Println("[2/2] Abrindo Cliente...")

	// Obter caminho absoluto para garantir que o Windows encontre o arquivo
	absClientPath, err := filepath.Abs("cliente/client.exe")
	if err != nil {
		log.Fatalf("Erro ao resolver caminho do cliente: %v", err)
	}

	clientCmd := exec.Command(absClientPath)
	clientCmd.Dir = "cliente" // Define o diretório de trabalho para carregar recursos (assets, etc)

	if err := clientCmd.Start(); err != nil {
		fmt.Printf("ERRO CRÍTICO: Não foi possível executar o cliente em %s\n", absClientPath)
		fmt.Printf("Detalhes: %v\n", err)
		fmt.Println("Pressione Enter para sair...")
		fmt.Scanln()
		return
	}

	fmt.Println("\nSucesso! FortressVision foi iniciado.")
	fmt.Println("O Launcher fechará automaticamente em 2 segundos...")
	time.Sleep(2 * time.Second)
}
