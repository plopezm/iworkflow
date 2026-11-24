//go:build linux && cgo && gtk3

package main

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1
#include <string.h>
#include <gtk/gtk.h>
#include <webkit2/webkit2.h>

// A certificate error is accepted only for a loopback host (the local server's self-signed certificate);
// every other host keeps WebKit's default rejection.
static gboolean iwf_on_tls_failed(WebKitWebView *view, const char *uri, GTlsCertificate *cert,
                                  GTlsCertificateFlags errors, gpointer data) {
	GUri *parsed = g_uri_parse(uri, G_URI_FLAGS_NONE, NULL);
	if (parsed == NULL) return FALSE;
	const char *host = g_uri_get_host(parsed);
	gboolean local = host != NULL && (strcmp(host, "127.0.0.1") == 0 || strcmp(host, "localhost") == 0 || strcmp(host, "::1") == 0);
	if (local) {
		webkit_web_context_allow_tls_certificate_for_host(webkit_web_view_get_context(view), cert, host);
		webkit_web_view_load_uri(view, uri);
	}
	g_uri_unref(parsed);
	return local;
}

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

// Returns 1 when the handler was installed.
static int iwf_trust_local_certificate(void *window) {
	WebKitWebView *view = NULL;
	iwf_find_webview(GTK_WIDGET(window), &view);
	if (view == NULL) return 0;
	g_signal_connect(view, "load-failed-with-tls-errors", G_CALLBACK(iwf_on_tls_failed), NULL);
	g_signal_connect(view, "decide-policy", G_CALLBACK(iwf_on_decide_policy), NULL);
	return 1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// trustLocalCertificate also sends external links to the system browser. It lets the window accept the local server's self-signed certificate. It must run on
// the GTK main thread, with the native GtkWindow of the window.
func trustLocalCertificate(window unsafe.Pointer, port int) error {
	C.iwf_set_server_port(C.int(port))
	if C.iwf_trust_local_certificate(window) == 0 {
		return errors.New("web view not found in the window")
	}
	return nil
}

// setTrustedPort tells the navigation guard the port the server now listens on (after a restart).
func setTrustedPort(port int) { C.iwf_set_server_port(C.int(port)) }
