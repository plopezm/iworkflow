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

// Returns 1 when the handler was installed.
static int iwf_trust_local_certificate(void *window) {
	WebKitWebView *view = NULL;
	iwf_find_webview(GTK_WIDGET(window), &view);
	if (view == NULL) return 0;
	g_signal_connect(view, "load-failed-with-tls-errors", G_CALLBACK(iwf_on_tls_failed), NULL);
	return 1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// trustLocalCertificate lets the window accept the local server's self-signed certificate. It must run on
// the GTK main thread, with the native GtkWindow of the window.
func trustLocalCertificate(window unsafe.Pointer) error {
	if C.iwf_trust_local_certificate(window) == 0 {
		return errors.New("web view not found in the window")
	}
	return nil
}
