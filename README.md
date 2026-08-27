# xmlrpcbrute
El script automatiza intentos de inicio de sesión contra un sitio WordPress mediante la interfaz XML-RPC, aprovechando la capacidad del método system.multicall para agrupar múltiples llamadas en una sola petición HTTP. Esto reduce drásticamente:

1. El número de conexiones de red necesarias.
2. El tiempo total del ataque.
3. La probabilidad de detección por límites de tasa (rate limiting) basados en IP.

### Mecanismo de Amplificación
Sin amplificación: Cada intento de contraseña requeriría una petición HTTP individual.

Con amplificación: `system.multicall` empaqueta hasta lote llamadas (ej. 50) en un solo XML. El servidor procesa todas internamente y devuelve un array de respuestas en una única respuesta HTTP.

Fórmula de reducción:

`Peticiones_Totales = (Total_Contraseñas / Tamaño_Lote) * Cantidad_Usuarios`

Ejemplo: 10,000 contraseñas × 5 usuarios = 50,000 peticiones → con lote=100, solo 500 peticiones.

### Parámetros

| **Parámetro** | **Tipo** | **Obligatorio** | **Descripción** |  
|---------------|----------|-----------------|-----------------|  
| `-url` | string | ✅ Sí | URL completa al archivo `xmlrpc.php` del sitio WordPress (ej. `https://ejemplo.com/xmlrpc.php`). |  
| `-usuarios` | string | ❌ No | Lista de nombres de usuario separada por comas (ej. `admin,editor,invitado`). *Valor por defecto:* `admin,root,user,test,wpadmin`. |  
| `-archivo` | string | ✅ Sí | Definir la wordlist con `-archivo /usr/share/wordlists/rockyou.txt`. |  
| `-hilos` | int | ❌ No | Número de goroutines concurrentes (cada una procesa un usuario). *Valor por defecto:* `5`. |  
| `-lote` | int | ❌ No | Cantidad de contraseñas por petición `system.multicall`. Ajustar según capacidad del servidor y red. *Valor por defecto:* `50`. |  
| `-retraso` | float | ❌ No | Pausa en segundos entre lotes para evitar sobrecarga y detección. *Valor por defecto:* `0.5`. |  

### Instalacion

```
go mod init xmlrpcbrute
```
```
go mod tidy
```
```
go build -o xmlrpcbrute
```
