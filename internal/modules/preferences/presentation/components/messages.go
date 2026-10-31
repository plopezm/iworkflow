package components

import "iworkflow/internal/core/i18n"

// User-visible texts of the network access card. They are copied from the Rust build, in the same
// five languages, so the screen reads the same after the migration.
var (
	loadingText = i18n.Msg{
		En: "Loading…",
		Es: "Cargando…",
		Zh: "加载中…",
		De: "Wird geladen…",
		Fr: "Chargement…",
	}
	cardTitle = i18n.Msg{
		En: "Network access",
		Es: "Acceso desde la red",
		Zh: "网络访问",
		De: "Netzwerkzugriff",
		Fr: "Accès réseau",
	}
	cardHelp = i18n.Msg{
		En: "By default iworkflow only listens on this computer. Enable network access to reach it from other devices, such as your phone. The change applies the next time the server starts.",
		Es: "Por defecto iworkflow solo escucha en este equipo. Activa el acceso desde la red para usarlo desde otros dispositivos, como el móvil. El cambio se aplica la próxima vez que arranque el servidor.",
		Zh: "默认情况下，iworkflow 只在本机监听。启用网络访问后，可从其他设备（例如手机）访问。更改将在服务器下次启动时生效。",
		De: "Standardmäßig lauscht iworkflow nur auf diesem Computer. Aktiviere den Netzwerkzugriff, um von anderen Geräten wie dem Smartphone darauf zuzugreifen. Die Änderung gilt ab dem nächsten Serverstart.",
		Fr: "Par défaut, iworkflow n'écoute que sur cet ordinateur. Activez l'accès réseau pour l'utiliser depuis d'autres appareils, comme votre téléphone. Le changement s'applique au prochain démarrage du serveur.",
	}
	warning = i18n.Msg{
		En: "iworkflow has no authentication and its agents and terminals run commands as your user. Anyone who can reach this port gets full control of this computer. Only enable it on a network you trust or a private network such as Tailscale.",
		Es: "iworkflow no tiene autenticación y sus agentes y terminales ejecutan comandos con tu usuario. Quien llegue a este puerto controla por completo este equipo. Actívalo solo en una red de confianza o privada, como Tailscale.",
		Zh: "iworkflow 没有身份验证，其智能体和终端会以你的用户身份执行命令。任何能访问此端口的人都能完全控制这台电脑。请仅在可信网络或 Tailscale 等私有网络中启用。",
		De: "iworkflow hat keine Authentifizierung, und seine Agenten und Terminals führen Befehle als dein Benutzer aus. Wer diesen Port erreicht, hat die volle Kontrolle über diesen Computer. Aktiviere es nur in einem vertrauenswürdigen oder privaten Netzwerk wie Tailscale.",
		Fr: "iworkflow n'a pas d'authentification et ses agents et terminaux exécutent des commandes avec votre utilisateur. Quiconque atteint ce port contrôle entièrement cet ordinateur. Activez-le seulement sur un réseau de confiance ou privé, comme Tailscale.",
	}
	toggleLabel = i18n.Msg{
		En: "Allow access from other devices on the network",
		Es: "Permitir el acceso desde otros dispositivos de la red",
		Zh: "允许网络中的其他设备访问",
		De: "Zugriff von anderen Geräten im Netzwerk erlauben",
		Fr: "Autoriser l'accès depuis d'autres appareils du réseau",
	}
	lockedByEnv = i18n.Msg{
		En: "The address is fixed by the --addr option or the IWORKFLOW_ADDR environment variable, so this setting has no effect.",
		Es: "La dirección está fijada por la opción --addr o la variable de entorno IWORKFLOW_ADDR, así que este ajuste no tiene efecto.",
		Zh: "地址由 --addr 选项或环境变量 IWORKFLOW_ADDR 固定，因此此设置不起作用。",
		De: "Die Adresse ist durch die Option --addr oder die Umgebungsvariable IWORKFLOW_ADDR festgelegt, daher hat diese Einstellung keine Wirkung.",
		Fr: "L'adresse est fixée par l'option --addr ou la variable d'environnement IWORKFLOW_ADDR ; ce réglage n'a donc aucun effet.",
	}
	restartPending = i18n.Msg{
		En: "Restart the server to apply the change.",
		Es: "Reinicia el servidor para aplicar el cambio.",
		Zh: "请重启服务器以应用更改。",
		De: "Starte den Server neu, um die Änderung zu übernehmen.",
		Fr: "Redémarrez le serveur pour appliquer le changement.",
	}
	restartNow = i18n.Msg{
		En: "Restart now",
		Es: "Reiniciar ahora",
		Zh: "立即重启",
		De: "Jetzt neu starten",
		Fr: "Redémarrer maintenant",
	}
	restarting = i18n.Msg{
		En: "Restarting… this page will reload when the server is back.",
		Es: "Reiniciando… esta página se recargará cuando el servidor vuelva.",
		Zh: "正在重启…服务器恢复后此页面将重新加载。",
		De: "Neustart läuft… Diese Seite wird neu geladen, sobald der Server wieder da ist.",
		Fr: "Redémarrage… cette page se rechargera quand le serveur sera de retour.",
	}
	statusExposed = i18n.Msg{
		En: "Reachable from the network",
		Es: "Accesible desde la red",
		Zh: "可通过网络访问",
		De: "Im Netzwerk erreichbar",
		Fr: "Accessible depuis le réseau",
	}
	statusPrivate = i18n.Msg{
		En: "Only this computer",
		Es: "Solo este equipo",
		Zh: "仅限本机",
		De: "Nur dieser Computer",
		Fr: "Cet ordinateur uniquement",
	}
	openFromPhone = i18n.Msg{
		En: "Open this address on your phone:",
		Es: "Abre esta dirección en el móvil:",
		Zh: "在手机上打开此地址：",
		De: "Öffne diese Adresse auf dem Smartphone:",
		Fr: "Ouvrez cette adresse sur votre téléphone :",
	}
	requireTokenLabel = i18n.Msg{
		En: "Require the access token (recommended)",
		Es: "Exigir el token de acceso (recomendado)",
		Zh: "要求访问令牌（推荐）",
		De: "Zugriffstoken verlangen (empfohlen)",
		Fr: "Exiger le jeton d'accès (recommandé)",
	}
	tokenOffWarning = i18n.Msg{
		En: "Without the token, anyone on the network can use iworkflow and run commands on this computer.",
		Es: "Sin el token, cualquiera en la red puede usar iworkflow y ejecutar comandos en este equipo.",
		Zh: "没有令牌时，网络中的任何人都可以使用 iworkflow 并在这台电脑上执行命令。",
		De: "Ohne Token kann jeder im Netzwerk iworkflow nutzen und Befehle auf diesem Computer ausführen.",
		Fr: "Sans le jeton, toute personne du réseau peut utiliser iworkflow et exécuter des commandes sur cet ordinateur.",
	}
	scanSignedIn = i18n.Msg{
		En: "Scan this code with your phone to open iworkflow already signed in:",
		Es: "Escanea este código con el móvil para abrir iworkflow ya autenticado:",
		Zh: "用手机扫描此二维码，即可以已登录状态打开 iworkflow：",
		De: "Scanne diesen Code mit dem Smartphone, um iworkflow bereits angemeldet zu öffnen:",
		Fr: "Scannez ce code avec votre téléphone pour ouvrir iworkflow déjà connecté :",
	}
	scanPlain = i18n.Msg{
		En: "Scan this code with your phone to open iworkflow:",
		Es: "Escanea este código con el móvil para abrir iworkflow:",
		Zh: "用手机扫描此二维码以打开 iworkflow：",
		De: "Scanne diesen Code mit dem Smartphone, um iworkflow zu öffnen:",
		Fr: "Scannez ce code avec votre téléphone pour ouvrir iworkflow :",
	}
	secureTitle = i18n.Msg{
		En: "HTTPS (recommended: the microphone works)",
		Es: "HTTPS (recomendado: el micrófono funciona)",
		Zh: "HTTPS（推荐：麦克风可用）",
		De: "HTTPS (empfohlen: das Mikrofon funktioniert)",
		Fr: "HTTPS (recommandé : le microphone fonctionne)",
	}
	plainTitle = i18n.Msg{
		En: "Plain HTTP (the microphone is not available)",
		Es: "HTTP plano (el micrófono no está disponible)",
		Zh: "普通 HTTP（麦克风不可用）",
		De: "Einfaches HTTP (das Mikrofon ist nicht verfügbar)",
		Fr: "HTTP simple (le microphone n'est pas disponible)",
	}
	interfaceLabel = i18n.Msg{
		En: "Network interface",
		Es: "Interfaz de red",
		Zh: "网络接口",
		De: "Netzwerkschnittstelle",
		Fr: "Interface réseau",
	}
	qrLabel = i18n.Msg{
		En: "QR code to open iworkflow from your phone",
		Es: "Código QR para abrir iworkflow desde el móvil",
		Zh: "用于在手机上打开 iworkflow 的二维码",
		De: "QR-Code zum Öffnen von iworkflow auf dem Smartphone",
		Fr: "Code QR pour ouvrir iworkflow depuis votre téléphone",
	}
	tokenSecretNote = i18n.Msg{
		En: "This link contains the access token: share it only with devices you trust.",
		Es: "Este enlace contiene el token de acceso: compártelo solo con dispositivos de confianza.",
		Zh: "此链接包含访问令牌：仅与你信任的设备共享。",
		De: "Dieser Link enthält das Zugriffstoken: Teile ihn nur mit vertrauenswürdigen Geräten.",
		Fr: "Ce lien contient le jeton d'accès : ne le partagez qu'avec des appareils de confiance.",
	}
	tokenLabel = i18n.Msg{
		En: "Access token",
		Es: "Token de acceso",
		Zh: "访问令牌",
		De: "Zugriffstoken",
		Fr: "Jeton d'accès",
	}
	regenerate = i18n.Msg{
		En: "Generate a new token",
		Es: "Generar un token nuevo",
		Zh: "生成新令牌",
		De: "Neues Token erzeugen",
		Fr: "Générer un nouveau jeton",
	}
	regenerateNote = i18n.Msg{
		En: "Devices that use the current token will have to sign in again.",
		Es: "Los dispositivos que usan el token actual tendrán que volver a autenticarse.",
		Zh: "使用当前令牌的设备需要重新登录。",
		De: "Geräte mit dem aktuellen Token müssen sich erneut anmelden.",
		Fr: "Les appareils qui utilisent le jeton actuel devront se reconnecter.",
	}
	portLabel = i18n.Msg{
		En: "Server port",
		Es: "Puerto del servidor",
		Zh: "服务器端口",
		De: "Server-Port",
		Fr: "Port du serveur",
	}
	portHelp = i18n.Msg{
		En: "Between 1024 and 65535. The change applies the next time the server starts.",
		Es: "Entre 1024 y 65535. El cambio se aplica la próxima vez que arranque el servidor.",
		Zh: "范围为 1024 至 65535。更改将在服务器下次启动时生效。",
		De: "Zwischen 1024 und 65535. Die Änderung gilt ab dem nächsten Serverstart.",
		Fr: "Entre 1024 et 65535. Le changement s'applique au prochain démarrage du serveur.",
	}
	portSave = i18n.Msg{
		En: "Save port",
		Es: "Guardar puerto",
		Zh: "保存端口",
		De: "Port speichern",
		Fr: "Enregistrer le port",
	}
)
