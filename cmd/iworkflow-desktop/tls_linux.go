//go:build linux && cgo && gtk3

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <stdlib.h>
#include <string.h>
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

static WebKitWebView *iwf_view = NULL;

static void iwf_find_webview(GtkWidget *widget, gpointer data) {
	WebKitWebView **found = (WebKitWebView **)data;
	if (*found != NULL) return;
	if (WEBKIT_IS_WEB_VIEW(widget)) { *found = WEBKIT_WEB_VIEW(widget); return; }
	if (GTK_IS_CONTAINER(widget)) gtk_container_forall(GTK_CONTAINER(widget), iwf_find_webview, data);
}

static int iwf_server_port = 0;

static gboolean iwf_is_local_host(const char *host) {
	return host != NULL && (strcmp(host, "127.0.0.1") == 0 || strcmp(host, "localhost") == 0 || strcmp(host, "::1") == 0);
}

// Navigations to anything but the local server open in the system browser instead of inside the app.
static gboolean iwf_on_decide_policy(WebKitWebView *view, WebKitPolicyDecision *decision,
                                     WebKitPolicyDecisionType type, gpointer data) {
	if (type != WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION && type != WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) return FALSE;
	WebKitNavigationPolicyDecision *nav = WEBKIT_NAVIGATION_POLICY_DECISION(decision);
	WebKitURIRequest *request = webkit_navigation_action_get_request(webkit_navigation_policy_decision_get_navigation_action(nav));
	const char *uri = webkit_uri_request_get_uri(request);
	GUri *parsed = g_uri_parse(uri, G_URI_FLAGS_NONE, NULL);
	if (parsed == NULL) return FALSE;
	const char *scheme = g_uri_get_scheme(parsed);
	gboolean web = scheme != NULL && (strcmp(scheme, "http") == 0 || strcmp(scheme, "https") == 0);
	gboolean local = iwf_is_local_host(g_uri_get_host(parsed)) && g_uri_get_port(parsed) == iwf_server_port;
	g_uri_unref(parsed);
	if (!web || local) {
		if (type == WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) { webkit_policy_decision_ignore(decision); return TRUE; }
		return FALSE;
	}
	g_app_info_launch_default_for_uri(uri, NULL, NULL);
	webkit_policy_decision_ignore(decision);
	return TRUE;
}

static void iwf_set_server_port(int port) { iwf_server_port = port; }

// Returns 1 when the guards were installed.
static int iwf_install_guards(void *window) {
	WebKitWebView *view = NULL;
	iwf_find_webview(GTK_WIDGET(window), &view);
	if (view == NULL) return 0;
	iwf_view = view;
	g_signal_connect(view, "decide-policy", G_CALLBACK(iwf_on_decide_policy), NULL);
	return 1;
}

// Tells WebKit to accept exactly this certificate for the host: no error page, no reload.
static int iwf_allow_certificate(const char *host, const char *pem) {
	if (iwf_view == NULL) return 0;
	GTlsCertificate *cert = g_tls_certificate_new_from_pem(pem, -1, NULL);
	if (cert == NULL) return 0;
	webkit_web_context_allow_tls_certificate_for_host(webkit_web_view_get_context(iwf_view), cert, host);
	g_object_unref(cert);
	return 1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// installWebViewGuards sends external links to the system browser. It must run on the GTK main thread, with
// the native GtkWindow of the window.
func installWebViewGuards(window unsafe.Pointer, port int) error {
	C.iwf_set_server_port(C.int(port))
	if C.iwf_install_guards(window) == 0 {
		return errors.New("web view not found in the window")
	}
	return nil
}

// allowServerCertificate makes the web view accept this PEM certificate for host (the local server's own
// self-signed certificate). Any other certificate keeps the default rejection. GTK main thread only.
func allowServerCertificate(host string, pem []byte) error {
	cHost, cPEM := C.CString(host), C.CString(string(pem))
	defer C.free(unsafe.Pointer(cHost))
	defer C.free(unsafe.Pointer(cPEM))
	if C.iwf_allow_certificate(cHost, cPEM) == 0 {
		return errors.New("certificate not accepted by the web view")
	}
	return nil
}

// setTrustedPort tells the navigation guard the port the server now listens on (after a restart).
func setTrustedPort(port int) { C.iwf_set_server_port(C.int(port)) }
