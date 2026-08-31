# xmlrpcbrute
Herramienta de fuerza bruta contra WordPress XML-RPC utilizando el método `system.multicall`. Permite enviar múltiples intentos de autenticación en una sola petición HTTP, optimizando el proceso de auditoría de credenciales.

## Características

- **Multicall**: Envía hasta 500 credenciales por petición HTTP usando `system.multicall` + `wp.getUsersBlogs`
- **Pool de workers**: Goroutines concurrentes controladas por `--workers`
- **Backoff exponencial**: Manejo inteligente de rate limiting (HTTP 429/503) con `2^attempt * cooldown`
- **Fallo individual automático**: Cuando WordPress devuelve falsos negativos en el multicall, verifica cada credencial individualmente
- **Progreso en tiempo real**: Muestra intentos/segundo, total, tiempo transcurrido y credenciales encontradas
- **Salida colorizada**: Verde para éxito, rojo para errores, amarillo para advertencias
- **Graceful shutdown**: Captura SIGINT/SIGTERM para terminación limpia
- **Test mode**: Modo integrado de verificación contra localhost

## Instalación

### Requisitos

- Go 1.21 o superior

### Compilación

```bash
git clone https://github.com/Euronymou5/xmlrpcbrute.git
cd xmlrpcbrute
go build -o xmlrpcbrute .
```

### Descarga de dependencias

```bash
go mod download
```

## Uso

```
  -t, --target=     URL del sitio WordPress (ej: http://localhost:8080)
  -u, --users=      Nombre de usuario o archivo con lista (uno por línea)
  -p, --passwords=  Archivo de lista de contraseñas (una por línea)
  -w, --workers=    Número de workers concurrentes (default: 10)
  -b, --batch-size= Credenciales por lote multicall (default: 50)
  -c, --cooldown=   Milisegundos de espera entre lotes (default: 100)
  -o, --output=     Archivo para guardar credenciales encontradas
      --test        Ejecutar prueba automática contra localhost:8080
  -v, --verbose     Mostrar intentos fallidos
```

### Ejemplos

```bash
# Prueba automática contra localhost
./xmlrpcbrute --target http://localhost:8080 \
  --passwords /home/kali/Downloads/2025-199_most_used_passwords.txt \
  --users admin --test

# Auditoría completa con múltiples usuarios
./xmlrpcbrute --target https://wordpress-site.com \
  --passwords rockyou.txt \
  --users usuarios.txt \
  --workers 20 \
  --batch-size 100 \
  --cooldown 200 \
  --output encontradas.txt

# Usuario único desde línea de comandos
./xmlrpcbrute -t https://ejemplo.com -p rockyou.txt -u admin

# Ajuste fino para entornos sensibles (más lento, más sigiloso)
./xmlrpcbrute -t https://ejemplo.com -p wordlist.txt -u admin -w 5 -b 25 -c 500 -v
```

## Arquitectura

```
xmlrpcbrute/
├── main.go         # Punto de entrada, orquestación, signal handling
├── config.go       # Estructuras de configuración, parsing de flags
├── wordpress.go    # Cliente XML-RPC, payloads, parseo de respuestas
├── bruteforce.go   # Pool de workers, batches, backoff, verificación
├── output.go       # Logging colorizado, progreso, escritura de resultados
├── go.mod          # Dependencias
└── go.sum          # Checksums de dependencias
```

## Cómo funciona

1. **Construcción del payload**: genera XML con `system.multicall` conteniendo N llamadas a `wp.getUsersBlogs`
2. **Envío**: cada worker toma un lote del canal y lo envía vía POST a `xmlrpc.php`
3. **Parseo**: divide la respuesta XML por etiquetas `<value><struct>` y clasifica cada resultado
4. **Detección de falsos negativos**: si todas las credenciales del lote vuelven como `faultCode`, se verifica cada una individualmente
5. **Verificación**: cada acierto se confirma con una llamada directa a `wp.getUsersBlogs`

## Limitaciones conocidas

- WordPress tiene una protección interna en `system.multicall`: si mezclas credenciales válidas e inválidas en un mismo lote, puede devolver `faultCode` incluso para las válidas. La herramienta mitiga esto con verificación individual automática.
- Algunos WAFs (Wordfence, Sucuri) pueden bloquear el endpoint `xmlrpc.php` o limitar el tamaño de los payloads.
- El método `system.multicall` puede no estar disponible en versiones muy antiguas de WordPress.

## Imagenes

<img width="800" height="449" alt="2026-08-3102-45-03" src="https://github.com/user-attachments/assets/7e9506ee-844d-4256-8860-a333d4cb580b" />
