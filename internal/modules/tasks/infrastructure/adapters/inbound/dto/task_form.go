package dto

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Boundary limits of the task forms. The body limit covers the images plus the fields, so an oversized upload
// is refused before it is read into memory.
const (
	MaxBaseBranchLength = 255
	maxFormBytes        = models.MaxImagesPerMessage*models.MaxImageBytes + 1<<20
	formMemoryBytes     = 1 << 20
	imagesField         = "images"
)

// Link kinds as the form sends them.
const (
	linkKindNext     = "next"
	linkKindParallel = "parallel"
)

// TaskForm is the create or edit form of a task as the browser sent it. Its fields are raw: ToNewTask and
// ToUpdate validate them and refuse the whole form on the first error, so nothing is stored from a rejected form.
type TaskForm struct {
	WorkspaceID string
	Title       string
	Prompt      string
	Priority    string
	BaseBranch  string
	MemoryLimit string
	CPULimit    string
	StartNow    bool
	// ShowAdvanced says the advanced options were open: the limits count only then (Rust parity). A hidden
	// limit is ignored, not validated, so the default from the settings applies.
	ShowAdvanced bool
	LinkKind     string
	LinkAnchor   string
	RemoveIDs    []string
	Images       []FormImage
}

// FormImage is an uploaded image. Its type is the one the bytes sniff to, not the one the browser declared.
type FormImage struct {
	Name     string
	MimeType string
	Data     []byte
}

// ErrFormTooLarge is returned when the form body is bigger than the upload limit.
var ErrFormTooLarge = errors.New("las imágenes superan el tamaño máximo del formulario")

// ParseTaskForm reads a multipart task form. The body is capped before parsing, and every image is read up to
// its limit: an image over the limit is refused with its name.
func ParseTaskForm(w http.ResponseWriter, r *http.Request) (TaskForm, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	values, files, err := readFormValues(r)
	if err != nil {
		return TaskForm{}, err
	}
	form := TaskForm{
		WorkspaceID:  first(values, "workspace_id"),
		Title:        first(values, "title"),
		Prompt:       first(values, "prompt"),
		Priority:     first(values, "priority"),
		BaseBranch:   first(values, "base_branch"),
		MemoryLimit:  first(values, "memory_limit_mb"),
		CPULimit:     first(values, "cpu_limit_percent"),
		StartNow:     first(values, "start_now") == "on",
		ShowAdvanced: first(values, "show_advanced") == "on",
		LinkKind:     first(values, "link_kind"),
		LinkAnchor:   first(values, "link_anchor"),
		RemoveIDs:    values["remove_attachment_ids"],
	}
	images, err := readImages(files[imagesField])
	if err != nil {
		return TaskForm{}, err
	}
	form.Images = images
	return form, nil
}

// readFormValues reads a multipart form, or a url-encoded one (without images). A body that is neither is refused.
func readFormValues(r *http.Request) (url.Values, map[string][]*multipart.FileHeader, error) {
	err := r.ParseMultipartForm(formMemoryBytes)
	if err == nil {
		return r.MultipartForm.Value, r.MultipartForm.File, nil
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return nil, nil, ErrFormTooLarge
	}
	if !errors.Is(err, http.ErrNotMultipart) {
		return nil, nil, fmt.Errorf("leer el formulario: %w", err)
	}
	mediaType, _, parseErr := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if parseErr != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, nil, errors.New("el formulario no es válido")
	}
	if err := r.ParseForm(); err != nil {
		return nil, nil, fmt.Errorf("leer el formulario: %w", err)
	}
	return r.PostForm, nil, nil
}

// first is the first value of a form field, or empty when the field was not sent.
func first(values map[string][]string, name string) string {
	if list := values[name]; len(list) > 0 {
		return strings.TrimSpace(list[0])
	}
	return ""
}

// readImages reads every uploaded image up to the image limit. An empty file input part is not an image, and
// the type of each image is sniffed from its bytes, so a file renamed to .png is judged by what it holds.
func readImages(headers []*multipart.FileHeader) ([]FormImage, error) {
	images := make([]FormImage, 0, len(headers))
	for _, header := range headers {
		if header.Filename == "" || header.Size == 0 {
			continue
		}
		if header.Size > models.MaxImageBytes {
			return nil, fmt.Errorf("%s: la imagen supera los 4 MiB", header.Filename)
		}
		data, err := readImage(header)
		if err != nil {
			return nil, err
		}
		images = append(images, FormImage{Name: header.Filename, MimeType: http.DetectContentType(data), Data: data})
	}
	return images, nil
}

// readImage reads one uploaded image, up to its limit: a file that grew past the limit is refused.
func readImage(header *multipart.FileHeader) ([]byte, error) {
	file, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("%s: no se pudo leer la imagen: %w", header.Filename, err)
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, models.MaxImageBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%s: no se pudo leer la imagen: %w", header.Filename, err)
	}
	if len(data) > models.MaxImageBytes {
		return nil, fmt.Errorf("%s: la imagen supera los 4 MiB", header.Filename)
	}
	return data, nil
}

// ToNewTask validates the create form and maps it to the domain input. A linked task carries its link, and its
// start request is left to the use case, which never starts a linked task.
func (f TaskForm) ToNewTask() (models.NewTask, *application.TaskLink, error) {
	if _, err := uuid.Parse(f.WorkspaceID); err != nil {
		return models.NewTask{}, nil, errors.New("el workspace no es válido")
	}
	input, err := f.baseTask()
	if err != nil {
		return models.NewTask{}, nil, err
	}
	link, err := f.parseLink()
	if err != nil {
		return models.NewTask{}, nil, err
	}
	images, err := f.imagesToDomain(0)
	if err != nil {
		return models.NewTask{}, nil, err
	}
	input.WorkspaceID = f.WorkspaceID
	input.Images = images
	input.StartNow = f.StartNow
	return input, link, nil
}

// ToUpdate validates the edit form and maps it to the update input. kept is how many stored images the task keeps
// (those not removed), so that the images it ends with stay within the limit.
func (f TaskForm) ToUpdate(kept int) (application.UpdateInput, error) {
	input, err := f.baseTask()
	if err != nil {
		return application.UpdateInput{}, err
	}
	images, err := f.imagesToDomain(kept)
	if err != nil {
		return application.UpdateInput{}, err
	}
	removed := make([]string, 0, len(f.RemoveIDs))
	for _, id := range f.RemoveIDs {
		if _, parseErr := uuid.Parse(id); parseErr != nil {
			return application.UpdateInput{}, errors.New("una imagen a quitar no es válida")
		}
		removed = append(removed, id)
	}
	return application.UpdateInput{
		Title:      input.Title,
		Prompt:     input.Prompt,
		Priority:   input.Priority,
		Limits:     input.Limits,
		BaseBranch: input.BaseBranch,
		Images:     images,
		RemoveIDs:  removed,
	}, nil
}

// baseTask validates the fields the create and the edit share: priority, limits and base branch. Title and
// prompt are validated by the domain, which owns their rules.
func (f TaskForm) baseTask() (models.NewTask, error) {
	var err error
	priority := models.DefaultTaskPriority
	if f.Priority != "" {
		parsed, err := models.ParseTaskPriority(f.Priority)
		if err != nil {
			return models.NewTask{}, err //nolint:wrapcheck // the domain error text is user-visible
		}
		priority = parsed
	}
	var memory, cpu *uint32
	if f.ShowAdvanced {
		memory, err = parseLimit(f.MemoryLimit, "la RAM máxima")
		if err != nil {
			return models.NewTask{}, err
		}
		cpu, err = parseLimit(f.CPULimit, "la CPU máxima")
		if err != nil {
			return models.NewTask{}, err
		}
	}
	branch, err := parseBaseBranch(f.BaseBranch)
	if err != nil {
		return models.NewTask{}, err
	}
	return models.NewTask{
		Title:      f.Title,
		Prompt:     f.Prompt,
		Priority:   priority,
		Limits:     models.ResourceLimits{MemoryMB: memory, CPUPercent: cpu},
		BaseBranch: branch,
	}, nil
}

// parseLimit reads an optional positive whole number. Empty means the default from the settings.
func parseLimit(raw string, label string) (*uint32, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || value == 0 {
		return nil, fmt.Errorf("%s debe ser un número entero mayor que cero", label)
	}
	limit := uint32(value)
	return &limit, nil
}

// parseBaseBranch checks the base branch. Empty means the workspace base branch. A name that starts with a dash
// would read as a git option, so it is refused.
func parseBaseBranch(raw string) (*string, error) {
	branch := strings.TrimSpace(raw)
	if branch == "" {
		return nil, nil
	}
	if len(branch) > MaxBaseBranchLength {
		return nil, fmt.Errorf("la rama base supera los %d caracteres", MaxBaseBranchLength)
	}
	if strings.HasPrefix(branch, "-") {
		return nil, errors.New("la rama base no es válida")
	}
	return &branch, nil
}

// parseLink reads the optional dependency of a new task. A kind needs an anchor, and an anchor needs a kind.
func (f TaskForm) parseLink() (*application.TaskLink, error) {
	if f.LinkKind == "" && f.LinkAnchor == "" {
		return nil, nil
	}
	anchor, err := models.ParseTaskID(f.LinkAnchor)
	if err != nil {
		return nil, errors.New("la tarea enlazada no es válida")
	}
	switch f.LinkKind {
	case linkKindNext:
		return &application.TaskLink{Kind: application.LinkNext, Anchor: anchor}, nil
	case linkKindParallel:
		return &application.TaskLink{Kind: application.LinkParallel, Anchor: anchor}, nil
	default:
		return nil, errors.New("el tipo de vínculo no es válido")
	}
}

// imagesToDomain validates the new images and encodes them as the domain expects. kept is the number of stored
// images the task keeps; the total must stay within models.MaxImagesPerMessage.
func (f TaskForm) imagesToDomain(kept int) ([]models.ImageAttachment, error) {
	if kept+len(f.Images) > models.MaxImagesPerMessage {
		return nil, fmt.Errorf("un mensaje admite como máximo %d imágenes", models.MaxImagesPerMessage)
	}
	images := make([]models.ImageAttachment, 0, len(f.Images))
	for _, image := range f.Images {
		validated, err := models.ImageAttachment{
			Name:     image.Name,
			MimeType: image.MimeType,
			Data:     base64.StdEncoding.EncodeToString(image.Data),
		}.ValidatedImage()
		if err != nil {
			return nil, err //nolint:wrapcheck // the domain error text is user-visible
		}
		images = append(images, validated)
	}
	return images, nil
}
