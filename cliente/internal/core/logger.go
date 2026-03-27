package core

import (
	"bufio"
	"io"
	"log"
	"os"
	"path/filepath"
)

// SetupLogger configura o log para sair tanto no console (CMD) quanto em um arquivo local,
// capturando inclusive logs fora do pacote padrão 'log' (ex: fmt.Print, slog da Kaiju).
func SetupLogger() {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Erro ao criar pasta de logs: %v", err)
		return
	}

	logFile := filepath.Join(logDir, "client.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Erro ao abrir arquivo de log: %v", err)
		return
	}

	// Salva as referências originais de Stdout e Stderr
	originalStdout := os.Stdout

	// Cria um pipe
	r, w, err := os.Pipe()
	if err == nil {
		os.Stdout = w
		os.Stderr = w

		// MultiWriter: escreve no Terminal (original) e no Arquivo
		mw := io.MultiWriter(originalStdout, f)

		// Goroutine copiando tudo que vai pro pipe para o terminal e arquivo
		go func() {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				mw.Write([]byte(scanner.Text() + "\n"))
			}
		}()
	}

	// Configurar o logger padrão do Go
	log.SetOutput(io.MultiWriter(originalStdout, f))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	
	log.Println("👷 [Mestre de Obras] Log do sistema inicializado. Vida longa à fortaleza!")
}
