package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"
)

type Config struct {
	Target    string `long:"target" short:"t" description:"URL del sitio WordPress (ej. http://localhost:8080)" required:"yes"`
	Users    string `long:"users" short:"u" description:"Nombre de usuario (string) o ruta a archivo con usuarios (uno por línea)" required:"yes"`
	Passwords string `long:"passwords" short:"p" description:"Ruta al archivo de lista de contraseñas (una por línea)" required:"yes"`
	Workers int   `long:"workers" short:"w" description:"Número de trabajadores concurrentes" default:"10"`
	BatchSize  int    `long:"batch-size" short:"b" description:"Credenciales por lote multicall" default:"50"`
	Cooldown int   `long:"cooldown" short:"c" description:"Milisegundos de espera entre lotes" default:"100"`
	Output      string `long:"output" short:"o" description:"Archivo para guardar credenciales encontradas"`
	TestMode  bool   `long:"test" description:"Ejecutar prueba automática contra localhost:8080"`
	Verbose     bool   `long:"verbose" short:"v" description:"Salida debug (mostrar fallos)"`
}

type ResolvedConfig struct {
	Target       string
	UsernameList []string
	PasswordList []string
	Workers      int
	BatchSize    int
	CooldownMs   int
	OutputFile   string
	TestMode     bool
	Verbose      bool
}

func ParseConfig() (*ResolvedConfig, error) {
	var cfg Config

	parser := flags.NewParser(&cfg, flags.Default)
	if _, err := parser.Parse(); err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "\nEjecuta con --help para ver la información de uso.\n")
		return nil, err
	}

	return resolveConfig(&cfg)
}

func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func resolveConfig(cfg *Config) (*ResolvedConfig, error) {
	target := strings.TrimRight(cfg.Target, "/")

	pwPath := ExpandPath(cfg.Passwords)
	if _, err := os.Stat(pwPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("archivo de contraseñas no encontrado: %s", pwPath)
	}

	passwordList, err := readLines(pwPath)
	if err != nil {
		return nil, fmt.Errorf("error al leer las contraseñas: %w", err)
	}
	if len(passwordList) == 0 {
		return nil, fmt.Errorf("el archivo de contraseñas esta vacio: %s", pwPath)
	}

	userPath := ExpandPath(cfg.Users)
	var usernameList []string

	if fi, err := os.Stat(userPath); err == nil && fi.Mode().IsRegular() {
		usernameList, err = readLines(userPath)
		if err != nil {
			return nil, fmt.Errorf("error al leer el archivo de usuarios: %w", err)
		}
		if len(usernameList) == 0 {
			return nil, fmt.Errorf("el archivo de usuarios está vacío: %s", userPath)
		}
	} else {
		if cfg.Users == "" {
			return nil, fmt.Errorf("se debe proporcionar un nombre de usuario mediante --users")
		}
		usernameList = []string{cfg.Users}
	}

	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1
	}
	if cfg.BatchSize > 500 {
		cfg.BatchSize = 500
	}

	if cfg.Workers < 1 {
		cfg.Workers = 1
	}

	return &ResolvedConfig{
		Target:       target,
		UsernameList: usernameList,
		PasswordList: passwordList,
		Workers:      cfg.Workers,
		BatchSize:    cfg.BatchSize,
		CooldownMs:   cfg.Cooldown,
		OutputFile:   cfg.Output,
		TestMode:     cfg.TestMode,
		Verbose:      cfg.Verbose,
	}, nil
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}
