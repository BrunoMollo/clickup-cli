# botty

TUI de terminal para ver tareas de los dos sprints más recientes de un proyecto de ClickUp.

## Requisitos

- Go 1.25 o superior.
- Token personal de ClickUp con acceso al proyecto.
- macOS, Linux o Windows con un navegador configurado.

Creá el token desde la configuración de ClickUp. La API oficial espera el token en el header `Authorization`: [ClickUp Authentication](https://developer.clickup.com/docs/authentication).

```sh
export CLICKUP_API_TOKEN='pk_...'
go run ./cmd/botty
```

El token nunca se guarda ni se acepta como flag.

## Configuración

La vista ancla predeterminada es:

```text
https://app.clickup.com/31037287/v/l/6-901417703320-1
```

Podés reemplazarla con una variable de entorno o flag:

```sh
export CLICKUP_ANCHOR_VIEW='https://app.clickup.com/31037287/v/l/6-901417703320-1'
go run ./cmd/botty

go run ./cmd/botty --anchor-view '6-901417703320-1'
go run ./cmd/botty --include-closed
```

Precedencia: `--anchor-view`, `CLICKUP_ANCHOR_VIEW`, valor predeterminado.

## Detección de sprints

1. Resuelve vista ancla con `GET /view/{view_id}`.
2. Obtiene carpeta desde lista padre de vista.
3. Carga listas no archivadas de carpeta.
4. Descarta listas sin `due_date` y sin `start_date`.
5. Ordena por `due_date`; usa `start_date` cuando no hay vencimiento.
6. Selecciona dos fechas máximas, incluso futuras.
7. Carga todas las páginas con tareas cerradas, subtareas y Tasks in Multiple Lists habilitadas.

Resolver implementa `sprint.Resolver`; puede reemplazarse sin cambiar TUI o servicio de tareas.

## Teclas

```text
↑ / k          tarea anterior
↓ / j          tarea siguiente
PgUp / PgDn    desplazar página
Espacio        expandir/colapsar subtareas
→ / l          expandir
← / h          colapsar o seleccionar padre
Enter          abrir tarea en navegador
a              alternar abiertas / abiertas + cerradas
r              recargar desde ClickUp
q / Ctrl+C     salir
```

Subtareas aparecen colapsadas. Al expandir, conservan estado propio. Subtarea abierta con padre cerrado se promueve temporalmente en modo `abiertas`.

## Compilar y verificar

```sh
make build
make check
```

Binario queda en `bin/botty`.

## Errores frecuentes

- `falta CLICKUP_API_TOKEN`: exportá token antes de iniciar.
- `se necesitan 2 listas con fecha`: configurá `start_date` o `due_date` en al menos dos listas de carpeta.
- `parent.type`: vista ancla debe pertenecer a una lista o carpeta.
- `HTTP 401/403`: token inválido o sin acceso.
- `HTTP 429`: cliente reintenta dos veces respetando límite de API.

API usada: [ClickUp API v2](https://developer.clickup.com/reference), [Get Tasks](https://developer.clickup.com/reference/gettasks), [Rate limits](https://developer.clickup.com/docs/rate-limits).
