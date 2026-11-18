# Móvil y Tailscale

> 🇬🇧 [English version](../mobile.md) · [← README](../../README.es.md)

iworkflow está diseñada **primero para el móvil**. Como tu trabajo es lanzar, seguir y revisar agentes (no escribir código), funciona bien desde un teléfono:

- La interfaz es responsive: en pantallas pequeñas, el menú lateral pasa a ser un panel desplegable desde una barra superior, y el espacio de trabajo con pestañas se adapta al viewport.
- El agente sigue ejecutándose en tu máquina mientras estás fuera. Desde el móvil consultas el progreso, respondes a las preguntas del agente, envías un mensaje, revisas el diff e integras o descartas.
- En iOS (Safari → Compartir → *Añadir a pantalla de inicio*) y en Android (Chrome → menú → *Añadir a la pantalla de inicio*) puedes fijarla como acceso directo. Incluye un manifiesto de aplicación web e iconos, así que se instala como una aplicación independiente a pantalla completa (Chrome y Edge también ofrecen *Instalar* en la barra de direcciones). No hay service worker: la aplicación necesita el servidor en marcha y no muestra copia sin conexión.

El servidor se ejecuta en tu ordenador y el móvil es solo un navegador, así que necesitas una forma segura de alcanzarlo: consulta la sección de Tailscale a continuación.

## Desarrollar desde el móvil con Tailscale

[Tailscale](https://tailscale.com) crea una red privada (tailnet) entre tus dispositivos, de modo que tu móvil puede alcanzar el servidor que se ejecuta en tu ordenador sin abrir puertos a internet. Esto es importante porque iworkflow solo se protege con un token compartido sobre HTTPS con un certificado autofirmado y lanza agentes que ejecutan comandos con tu usuario.

1. Instala Tailscale en el ordenador que ejecuta iworkflow y en el móvil (App Store / Google Play), e inicia sesión con la **misma cuenta** en ambos.
2. Arranca el servidor escuchando en la interfaz de Tailscale (o en todas, que es el valor por defecto de `make dist` / `make run`):

   ```bash
   make run                                        # bind a 0.0.0.0:9000
   # o solo accesible desde esta máquina y, mediante Tailscale Serve, desde la tailnet (ver abajo)
   make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
   ```

3. Obtén el nombre o la IP del ordenador en la tailnet:

   ```bash
   tailscale ip -4        # p. ej. 100.101.102.103
   tailscale status       # muestra el nombre MagicDNS, p. ej. mipc.nombre-tailnet.ts.net
   ```

4. En el móvil, abre `https://<ip-tailnet>:9000` o `https://mipc.nombre-tailnet.ts.net:9000` (acepta el certificado autofirmado una vez).

**Recomendado: HTTPS con Tailscale Serve.** Mantén el bind en `127.0.0.1` y deja que Tailscale lo exponga solo dentro de tu tailnet, con un certificado válido:

```bash
tailscale serve --bg 3000
# https://mipc.nombre-tailnet.ts.net  (solo accesible desde tus dispositivos de la tailnet)
```

Los WebSockets del terminal se aceptan cuando el host de `Origin` coincide con la cabecera `Host`, lo que ocurre si usas la misma dirección en el navegador.

Consejos:

- **No** uses Tailscale Funnel: publicaría la aplicación en todo internet.
- Limita con las ACL de Tailscale quién puede llegar a la máquina si la tailnet es compartida.
- El ordenador debe seguir encendido y el servidor en marcha; usa `tmux` o un servicio de usuario de systemd para las sesiones largas.
