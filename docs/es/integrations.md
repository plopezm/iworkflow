# Integraciones

> 🇬🇧 [English version](../integrations.md) · [← README](../../README.es.md)

## Integración con GitLab y GitHub

Desde cada tarea puedes crear un merge request (GitLab) o pull request (GitHub), seguir su CI y mezclarlo. Un workspace puede ser un repositorio de cualquiera de los dos: el proveedor se detecta por el host del remoto `origin` y se aplica la configuración de ese proveedor.

1. **Configuración** tiene una tarjeta por proveedor.
   - **GitLab**: la URL de tu instancia (p. ej. `https://gitlab.miempresa.local`) y un token personal con el scope `api`.
   - **GitHub**: `https://github.com` (propuesta por defecto) o la URL de tu GitHub Enterprise, y un token personal (scope `repo`; de grano fino: Pull requests y Contents en lectura/escritura, Actions en lectura).
   Usa "Probar conexión" para validarlos. Si tu certificado viene de una CA interna o es autofirmado, marca "Aceptar certificado TLS no confiable".
2. El proyecto se deduce del remoto `origin` (https, ssh o scp). Su host debe coincidir con la URL de un proveedor configurado; si no coincide con ninguno se te avisa, y si es github.com y GitHub no está configurado se te pide configurarlo. En GitHub, el repositorio es `propietario/repositorio`.
3. En el panel de la tarea, pestaña **PR / MR**: crear el MR/PR (rama de la tarea → rama base), ver su estado y el último pipeline (GitLab) o workflow (GitHub) con sus jobs, y **Merge** (pide confirmación).

Notas:
- Crear el MR/PR publica la rama de la tarea en `origin` con `git push` si todavía no está. El push va a `<URL del proveedor>/<proyecto>.git` por HTTPS y se autentica con el token configurado aquí (HTTP Basic, usuario `x-access-token` en GitHub y `oauth2` en GitLab), así que funciona aunque `origin` sea un remoto SSH y no necesita credenciales git propias. El token se entrega a git por el entorno del proceso (nunca por la línea de comandos), solo para esa instancia, e ignora cualquier credential helper.
- El destino es la rama base de la tarea **tal como se llama en el servidor**: una referencia remota como `origin/develop` se envía como `develop`. Si esa rama no está en `origin`, o es la propia rama de la tarea, la creación se rechaza con un mensaje explícito en lugar del error bruto del proveedor.
- El token se guarda en claro en la base de datos local (`workspaces.db` en la carpeta de datos) y nunca se envía al navegador. Dejarlo vacío al editar conserva el guardado, salvo que cambies la URL: entonces hay que volver a introducirlo, para no enviarlo a otro servidor.
- Las redirecciones HTTP están desactivadas para que el token no se filtre a otro host.

## Integración con Jira (Server / Data Center)

La pestaña **Jira** se conecta a un Jira on premise con la URL del servidor y un token de acceso personal (PAT, `Authorization: Bearer`, API REST v2). Al guardar, la conexión se valida contra Jira; el token se guarda en la base de datos local y nunca vuelve al navegador (dejarlo vacío al editar conserva el guardado).

- **Importar**: «Sincronizar» trae las incidencias que cumplen el JQL de la conexión (por defecto `assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC`, como máximo 500). El estado local (nueva / convertida / ignorada) se conserva entre sincronizaciones; las incidencias nuevas que Jira ya no devuelve se retiran.
- **Gestionar**: lista con búsqueda y filtros, detalle con descripción y comentarios, y escritura hacia Jira: cambiar el estado (transiciones del workflow) y comentar. Ignorar y restaurar son solo locales.
- **Convertir**: el botón «Convertir en tarea» abre un formulario con el título, el prompt y la prioridad propuestos a partir de la incidencia de Jira (la prioridad se traduce desde el nombre de Jira); eliges workspace y rama base, editas lo que quieras y se crea la tarea (opcionalmente iniciada). La incidencia de Jira queda marcada como convertida y enlazada.
- Si el servidor usa un certificado autofirmado o de una CA interna que el sistema no conoce, marca «Aceptar certificado no verificado» en la conexión.

## Perfiles y modelos de agentes

Esta funcionalidad es la integración con [Gentle AI](https://github.com/Gentleman-Programming/gentle-ai), el arnés de agentes que hay detrás de `gentle-shell` y de su paquete gentle-pi. Los perfiles y el enrutado de modelos por agente son conceptos de Gentle AI: iworkflow no define los suyos, sino que lee y escribe los mismos ficheros de configuración (`profiles.json`, `models.json`), así que los cambios hechos aquí los ve la TUI de Gentle AI y viceversa. Instala y configura Gentle AI primero; sin él, estas pantallas no tienen nada que gestionar.

Dos pantallas equivalentes a los comandos de gentle-pi, que trabajan sobre los mismos ficheros:

| Pantalla | Equivale a | Qué hace |
|---|---|---|
| `/profiles` | `/gentle:profiles` | Crear, duplicar, renombrar, eliminar, importar y exportar perfiles (`profiles.json`), editar el modelo y el esfuerzo por agente y crear roles de agente nuevos. |
| `/models` | `/gentle:models` | Editar el enrutado global (`models.json`): modelo y esfuerzo por agente. |

Los perfiles se eligen en el desplegable de ajustes del agente, que lee el mismo `profiles.json`. Estas pantallas no activan un perfil de forma global; solo lo gestionan. El bloqueo de escritura evita pisadas dentro de un mismo proceso, pero no frente a otro proceso (p. ej. el TUI de gentle-pi) que edite a la vez.

### Memoria de Engram

Desactivada por defecto. Con `IWF_ENGRAM_URL=http://127.0.0.1:7437` (o la dirección de tu Engram), iworkflow muestra, en cada workspace, cuántas memorias guarda Engram para su proyecto (la etiqueta en Workspaces), y, en cada tarea, el tipo, el título y la fecha de sus cinco memorias más recientes (el panel Memoria). El acceso es de solo lectura: iworkflow solo envía peticiones `GET` (`/projects` y `/observations/recent`) y nunca guarda, edita ni borra memorias. El contenido de una memoria nunca se muestra, solo su tipo, su título y su fecha. Cuando Engram no responde, las pantallas lo indican como no disponible y siguen funcionando. Sin la variable, o con una URL no válida, la función está desactivada y iworkflow no envía ninguna petición a Engram.
