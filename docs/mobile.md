# Mobile and Tailscale

> 🇪🇸 [Versión en castellano](es/mobile.md) · [← README](../README.md)

iworkflow is built **mobile-first**. Since your job is to launch, follow and review agents (not to type code), it works well from a phone:

- The UI is responsive: on small screens the side menu becomes a drawer opened from a top bar, and the tabbed workspace adapts to the viewport.
- The agent keeps running on your machine while you are away. From the phone you check progress, answer agent questions, send a message, review the diff and merge or discard.
- On iOS (Safari → Share → *Add to Home Screen*) and Android (Chrome → menu → *Add to Home screen*) you can pin it as a shortcut. It ships a web app manifest and icons, so it installs as a standalone full-screen app (Chrome and Edge also offer *Install* in the address bar). There is no service worker: the app needs the server running and shows no offline copy.

The server runs on your computer and the phone is only a browser, so you need a secure way to reach it: see [Developing from your phone with Tailscale](#developing-from-your-phone-with-tailscale).

[Tailscale](https://tailscale.com) creates a private network (tailnet) between your devices, so your phone can reach the server running on your computer without opening ports to the internet. This matters because iworkflow is protected only by a shared token over plain HTTP and launches agents that run commands as your user.

1. Install Tailscale on the computer that runs iworkflow and on the phone (App Store / Google Play), and sign in to the **same account** on both.
2. Start the server listening on the Tailscale interface (or all interfaces, which is the default of `make dist` / `make run`):

   ```bash
   make run                                        # binds 0.0.0.0:9000
   # or only reachable from this machine and the tailnet via Tailscale Serve (see below)
   make run SITE_ADDR=127.0.0.1:3000 ARGS=--no-open
   ```

3. Find the computer's tailnet name or IP:

   ```bash
   tailscale ip -4        # e.g. 100.101.102.103
   tailscale status       # shows the MagicDNS name, e.g. mypc.tailnet-name.ts.net
   ```

4. On the phone, open `http://<tailnet-ip>:9000` or `http://mypc.tailnet-name.ts.net:9000`.

**Recommended: HTTPS with Tailscale Serve.** Keep the bind on `127.0.0.1` and let Tailscale expose it only inside your tailnet, with a valid certificate:

```bash
tailscale serve --bg 3000
# https://mypc.tailnet-name.ts.net  (only reachable by your tailnet devices)
```

Terminal WebSockets are accepted when the `Origin` host matches the `Host` header, which holds when you use the same address in the browser.

Tips:

- Do **not** use Tailscale Funnel: it would publish the app to the whole internet.
- Restrict who can reach the machine with Tailscale ACLs if the tailnet is shared.
- The computer must stay awake and the server running; use `tmux`/a systemd user service for long sessions.
