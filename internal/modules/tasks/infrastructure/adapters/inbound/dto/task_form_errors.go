package dto

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// FormError is a form refusal that can be shown in the language of the request. Error is the Spanish text of
// the Rust version, kept for logs and for callers that only read the message; the handlers show Text instead.
type FormError struct {
	// Message is the text in every language; its {name} placeholders are filled from Args.
	Message i18n.Msg
	Args    map[string]string
	// Cause is the error underneath, when there is one; its text fills the {error} placeholder.
	Cause error
}

// Error returns the Spanish text.
func (e *FormError) Error() string {
	return e.Text(i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es}))
}

// Unwrap exposes the underlying error to errors.Is and errors.As.
func (e *FormError) Unwrap() error { return e.Cause }

// Text returns the message in the language of the request carried by ctx.
func (e *FormError) Text(ctx context.Context) string {
	args := make(map[string]string, len(e.Args)+1)
	for name, value := range e.Args {
		args[name] = value
	}
	if e.Cause != nil {
		args["error"] = e.Cause.Error()
	}
	return e.Message.Fmt(ctx, args)
}

// formErr builds a form error from its message and placeholder values.
func formErr(message i18n.Msg, args map[string]string) *FormError {
	return &FormError{Message: message, Args: args}
}

// Boundary messages of the task forms. The Spanish texts are the ones of the Rust version, byte for byte.
var (
	msgFormTooLarge = i18n.Msg{
		En: "The images exceed the maximum size of the form",
		Es: "las imágenes superan el tamaño máximo del formulario",
		Zh: "图片超过表单的最大大小",
		De: "Die Bilder überschreiten die maximale Größe des Formulars",
		Fr: "Les images dépassent la taille maximale du formulaire",
	}
	msgFormUnreadable = i18n.Msg{
		En: "The form could not be read: {error}",
		Es: "leer el formulario: {error}",
		Zh: "无法读取表单：{error}",
		De: "Das Formular konnte nicht gelesen werden: {error}",
		Fr: "Le formulaire n’a pas pu être lu : {error}",
	}
	msgFormInvalid = i18n.Msg{
		En: "The form is not valid",
		Es: "el formulario no es válido",
		Zh: "表单无效",
		De: "Das Formular ist ungültig",
		Fr: "Le formulaire n’est pas valide",
	}
	msgImageTooBig = i18n.Msg{
		En: "{name}: the image exceeds {mib} MiB",
		Es: "{name}: la imagen supera los {mib} MiB",
		Zh: "{name}：图片超过 {mib} MiB",
		De: "{name}: Das Bild überschreitet {mib} MiB",
		Fr: "{name} : l’image dépasse {mib} Mio",
	}
	msgImageUnreadable = i18n.Msg{
		En: "{name}: the image could not be read: {error}",
		Es: "{name}: no se pudo leer la imagen: {error}",
		Zh: "{name}：无法读取图片：{error}",
		De: "{name}: Das Bild konnte nicht gelesen werden: {error}",
		Fr: "{name} : l’image n’a pas pu être lue : {error}",
	}
	msgWorkspaceInvalid = i18n.Msg{
		En: "The workspace is not valid",
		Es: "el workspace no es válido",
		Zh: "工作区无效",
		De: "Der Workspace ist ungültig",
		Fr: "L’espace de travail n’est pas valide",
	}
	msgRemovedImageInvalid = i18n.Msg{
		En: "An image to remove is not valid",
		Es: "una imagen a quitar no es válida",
		Zh: "要移除的图片无效",
		De: "Ein zu entfernendes Bild ist ungültig",
		Fr: "Une image à retirer n’est pas valide",
	}
	msgRAMLimitInvalid = i18n.Msg{
		En: "The maximum RAM must be a whole number greater than zero",
		Es: "la RAM máxima debe ser un número entero mayor que cero",
		Zh: "最大内存必须是大于零的整数",
		De: "Der maximale RAM muss eine ganze Zahl größer als null sein",
		Fr: "La RAM max. doit être un entier supérieur à zéro",
	}
	msgCPULimitInvalid = i18n.Msg{
		En: "The maximum CPU must be a whole number greater than zero",
		Es: "la CPU máxima debe ser un número entero mayor que cero",
		Zh: "最大 CPU 必须是大于零的整数",
		De: "Die maximale CPU muss eine ganze Zahl größer als null sein",
		Fr: "Le CPU max. doit être un entier supérieur à zéro",
	}
	msgBranchTooLong = i18n.Msg{
		En: "The base branch exceeds {max} characters",
		Es: "la rama base supera los {max} caracteres",
		Zh: "基础分支超过 {max} 个字符",
		De: "Der Basis-Branch ist länger als {max} Zeichen",
		Fr: "La branche de base dépasse {max} caractères",
	}
	msgBranchInvalid = i18n.Msg{
		En: "The base branch is not valid",
		Es: "la rama base no es válida",
		Zh: "基础分支无效",
		De: "Der Basis-Branch ist ungültig",
		Fr: "La branche de base n’est pas valide",
	}
	msgLinkedTaskInvalid = i18n.Msg{
		En: "The linked task is not valid",
		Es: "la tarea enlazada no es válida",
		Zh: "关联的任务无效",
		De: "Die verknüpfte Aufgabe ist ungültig",
		Fr: "La tâche liée n’est pas valide",
	}
	msgLinkKindInvalid = i18n.Msg{
		En: "The link type is not valid",
		Es: "el tipo de vínculo no es válido",
		Zh: "链接类型无效",
		De: "Der Verknüpfungstyp ist ungültig",
		Fr: "Le type de lien n’est pas valide",
	}
	msgTooManyImages = i18n.Msg{
		En: "A message admits at most {max} images",
		Es: "un mensaje admite como máximo {max} imágenes",
		Zh: "每条消息最多可附加 {max} 张图片",
		De: "Eine Nachricht erlaubt höchstens {max} Bilder",
		Fr: "Un message admet au plus {max} images",
	}
)
