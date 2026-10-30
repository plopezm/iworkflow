//nolint:contextcheck // templ components receive the request context at Render time; the linter cannot see through the closures
package inbound

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/pages"
	"iworkflow/internal/modules/jira/presentation/state"
)

// IssueHandlers serves the Jira issue detail, action, and conversion endpoints.
type IssueHandlers struct {
	GetIssue               *application.GetIssue
	ListComments           *application.ListIssueComments
	ListTransitions        *application.ListIssueTransitions
	AddComment             *application.AddIssueComment
	TransitionIssue        *application.TransitionIssue
	SetIssueIgnored        *application.SetIssueIgnored
	PrepareConversion      *application.PrepareConversion
	ListConversionBranches *application.ListConversionBranches
	ConvertIssue           *application.ConvertIssue
}

// NewIssueHandlers constructs IssueHandlers with all required use cases.
func NewIssueHandlers(
	getIssue *application.GetIssue,
	listComments *application.ListIssueComments,
	listTransitions *application.ListIssueTransitions,
	addComment *application.AddIssueComment,
	transitionIssue *application.TransitionIssue,
	setIssueIgnored *application.SetIssueIgnored,
	prepareConversion *application.PrepareConversion,
	listConversionBranches *application.ListConversionBranches,
	convertIssue *application.ConvertIssue,
) *IssueHandlers {
	return &IssueHandlers{
		GetIssue:               getIssue,
		ListComments:           listComments,
		ListTransitions:        listTransitions,
		AddComment:             addComment,
		TransitionIssue:        transitionIssue,
		SetIssueIgnored:        setIssueIgnored,
		PrepareConversion:      prepareConversion,
		ListConversionBranches: listConversionBranches,
		ConvertIssue:           convertIssue,
	}
}

// Mount registers the issue comments, transitions, ignore, and conversion fragment routes.
// The pages themselves (/jira/{key} and /jira/{key}/convert) are served as dock tabs.
func (h *IssueHandlers) Mount(r chi.Router) {
	r.Post("/jira/{key}/comments", h.AddCommentHandler)
	r.Post("/jira/{key}/transitions", h.TransitionHandler)
	r.Post("/jira/{key}/ignore", h.IgnoreHandler)
	r.Get("/jira/{key}/convert/branches", h.GetBranches)
	r.Post("/jira/{key}/convert", h.ConvertHandler)
}

// IssuePage loads an issue, its comments and transitions, returning the Templ detail component.
func (h *IssueHandlers) IssuePage(ctx context.Context, key string) templ.Component {
	issue, err := h.GetIssue.Execute(ctx, key)
	if err != nil {
		return pages.JiraIssuePage(state.IssueDetailView{
			Issue:     dto.IssueDTO{Key: key},
			ActionErr: err.Error(),
		})
	}

	var commentsDTO []dto.CommentDTO
	var commentsErr string
	comments, cErr := h.ListComments.Execute(ctx, key)
	if cErr != nil {
		commentsErr = cErr.Error()
	} else {
		commentsDTO = dto.CommentsFromDomain(comments)
	}

	var transitionsDTO []dto.TransitionDTO
	var transitionsErr string
	transitions, tErr := h.ListTransitions.Execute(ctx, key)
	if tErr != nil {
		transitionsErr = tErr.Error()
	} else {
		transitionsDTO = dto.TransitionsFromDomain(transitions)
	}

	view := state.IssueDetailView{
		Issue:          dto.IssueFromDomain(issue),
		Comments:       commentsDTO,
		CommentsErr:    commentsErr,
		Transitions:    transitionsDTO,
		TransitionsErr: transitionsErr,
	}
	return pages.JiraIssuePage(view)
}

// GetIssuePage handles GET /jira/{key}.
func (h *IssueHandlers) GetIssuePage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	_, err := h.GetIssue.Execute(r.Context(), key)
	if err != nil {
		if errors.Is(err, jiraerrors.IssueNotFound(key)) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	web.Render(w, r, http.StatusOK, h.IssuePage(r.Context(), key))
}

// AddCommentHandler handles POST /jira/{key}/comments.
func (h *IssueHandlers) AddCommentHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	body := r.FormValue("body")

	err := h.AddComment.Execute(r.Context(), key, body)
	if err != nil {
		issue, iErr := h.GetIssue.Execute(r.Context(), key)
		var issueDTO dto.IssueDTO
		if iErr == nil {
			issueDTO = dto.IssueFromDomain(issue)
		} else {
			issueDTO = dto.IssueDTO{Key: key}
		}

		var commentsDTO []dto.CommentDTO
		var commentsErr string
		comments, cErr := h.ListComments.Execute(r.Context(), key)
		if cErr != nil {
			commentsErr = cErr.Error()
		} else {
			commentsDTO = dto.CommentsFromDomain(comments)
		}

		var transitionsDTO []dto.TransitionDTO
		var transitionsErr string
		transitions, tErr := h.ListTransitions.Execute(r.Context(), key)
		if tErr != nil {
			transitionsErr = tErr.Error()
		} else {
			transitionsDTO = dto.TransitionsFromDomain(transitions)
		}

		view := state.IssueDetailView{
			Issue:          issueDTO,
			Comments:       commentsDTO,
			CommentsErr:    commentsErr,
			Transitions:    transitionsDTO,
			TransitionsErr: transitionsErr,
			ActionErr:      err.Error(),
			CommentText:    body,
		}
		web.Render(w, r, http.StatusOK, pages.JiraIssuePage(view))
		return
	}

	web.Render(w, r, http.StatusOK, h.IssuePage(r.Context(), key))
}

// TransitionHandler handles POST /jira/{key}/transitions.
func (h *IssueHandlers) TransitionHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	transitionID := r.FormValue("transition_id")

	_, err := h.TransitionIssue.Execute(r.Context(), key, transitionID)
	if err != nil {
		issue, iErr := h.GetIssue.Execute(r.Context(), key)
		var issueDTO dto.IssueDTO
		if iErr == nil {
			issueDTO = dto.IssueFromDomain(issue)
		} else {
			issueDTO = dto.IssueDTO{Key: key}
		}

		var commentsDTO []dto.CommentDTO
		var commentsErr string
		comments, cErr := h.ListComments.Execute(r.Context(), key)
		if cErr != nil {
			commentsErr = cErr.Error()
		} else {
			commentsDTO = dto.CommentsFromDomain(comments)
		}

		var transitionsDTO []dto.TransitionDTO
		var transitionsErr string
		transitions, tErr := h.ListTransitions.Execute(r.Context(), key)
		if tErr != nil {
			transitionsErr = tErr.Error()
		} else {
			transitionsDTO = dto.TransitionsFromDomain(transitions)
		}

		view := state.IssueDetailView{
			Issue:              issueDTO,
			Comments:           commentsDTO,
			CommentsErr:        commentsErr,
			Transitions:        transitionsDTO,
			TransitionsErr:     transitionsErr,
			ActionErr:          err.Error(),
			SelectedTransition: transitionID,
		}
		web.Render(w, r, http.StatusOK, pages.JiraIssuePage(view))
		return
	}

	web.Render(w, r, http.StatusOK, h.IssuePage(r.Context(), key))
}

// IgnoreHandler handles POST /jira/{key}/ignore.
func (h *IssueHandlers) IgnoreHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	ignoredParam := r.URL.Query().Get("ignored")
	if ignoredParam == "" {
		ignoredParam = r.FormValue("ignored")
	}
	ignored := ignoredParam == "true" || ignoredParam == "1"

	err := h.SetIssueIgnored.Execute(r.Context(), key, ignored)
	if err != nil {
		issue, iErr := h.GetIssue.Execute(r.Context(), key)
		var issueDTO dto.IssueDTO
		if iErr == nil {
			issueDTO = dto.IssueFromDomain(issue)
		} else {
			issueDTO = dto.IssueDTO{Key: key}
		}

		var commentsDTO []dto.CommentDTO
		var commentsErr string
		comments, cErr := h.ListComments.Execute(r.Context(), key)
		if cErr != nil {
			commentsErr = cErr.Error()
		} else {
			commentsDTO = dto.CommentsFromDomain(comments)
		}

		var transitionsDTO []dto.TransitionDTO
		var transitionsErr string
		transitions, tErr := h.ListTransitions.Execute(r.Context(), key)
		if tErr != nil {
			transitionsErr = tErr.Error()
		} else {
			transitionsDTO = dto.TransitionsFromDomain(transitions)
		}

		view := state.IssueDetailView{
			Issue:          issueDTO,
			Comments:       commentsDTO,
			CommentsErr:    commentsErr,
			Transitions:    transitionsDTO,
			TransitionsErr: transitionsErr,
			ActionErr:      err.Error(),
		}
		web.Render(w, r, http.StatusOK, pages.JiraIssuePage(view))
		return
	}

	web.Render(w, r, http.StatusOK, h.IssuePage(r.Context(), key))
}

// ConvertPage loads the conversion preparation and initial workspace branches.
func (h *IssueHandlers) ConvertPage(ctx context.Context, key string) templ.Component {
	prep, err := h.PrepareConversion.Execute(ctx, key)
	if err != nil {
		return pages.JiraConvertPage(state.ConvertIssueView{
			Preparation: dto.ConversionPreparationDTO{
				Issue: dto.IssueDTO{Key: key},
			},
			Error: err.Error(),
		})
	}

	prepDTO := dto.PreparationFromDomain(prep)
	var branchesDTO []dto.ConversionBranchDTO
	if len(prep.Workspaces) > 0 {
		branches, _ := h.ListConversionBranches.Execute(ctx, prep.Workspaces[0].ID)
		branchesDTO = dto.BranchesFromDomain(branches)
	}

	return pages.JiraConvertPage(state.ConvertIssueView{
		Preparation: prepDTO,
		Branches:    branchesDTO,
	})
}

// GetConvertPage handles GET /jira/{key}/convert.
func (h *IssueHandlers) GetConvertPage(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	_, err := h.GetIssue.Execute(r.Context(), key)
	if err != nil {
		if errors.Is(err, jiraerrors.IssueNotFound(key)) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	web.Render(w, r, http.StatusOK, h.ConvertPage(r.Context(), key))
}

// GetBranches handles GET /jira/{key}/convert/branches.
func (h *IssueHandlers) GetBranches(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	branches, _ := h.ListConversionBranches.Execute(r.Context(), workspaceID)
	branchesDTO := dto.BranchesFromDomain(branches)
	web.Render(w, r, http.StatusOK, pages.ConversionBranchContainer(branchesDTO))
}

// ConvertHandler handles POST /jira/{key}/convert.
func (h *IssueHandlers) ConvertHandler(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	workspaceID := r.FormValue("workspace_id")
	baseBranch := r.FormValue("base_branch")
	title := r.FormValue("title")
	prompt := r.FormValue("prompt")
	priority := r.FormValue("priority")
	startNow := r.FormValue("start_now") == "true" || r.FormValue("start_now") == "on" || r.FormValue("start_now") == "1"

	var baseBranchPtr *string
	if strings.TrimSpace(baseBranch) != "" {
		trimmed := strings.TrimSpace(baseBranch)
		baseBranchPtr = &trimmed
	}

	convertDTO := dto.ConvertIssueDTO{
		Key:         key,
		WorkspaceID: strings.TrimSpace(workspaceID),
		BaseBranch:  baseBranchPtr,
		Title:       title,
		Prompt:      prompt,
		Priority:    priority,
		StartNow:    startNow,
	}

	converted, err := h.ConvertIssue.Execute(r.Context(), convertDTO.ToConversionRequest())
	if err != nil {
		prep, pErr := h.PrepareConversion.Execute(r.Context(), key)
		var prepDTO dto.ConversionPreparationDTO
		if pErr == nil {
			prepDTO = dto.PreparationFromDomain(prep)
		} else {
			prepDTO = dto.ConversionPreparationDTO{Issue: dto.IssueDTO{Key: key}}
		}
		prepDTO.Title = title
		prepDTO.Prompt = prompt
		prepDTO.Priority = priority

		targetWS := strings.TrimSpace(workspaceID)
		if targetWS == "" && len(prepDTO.Workspaces) > 0 {
			targetWS = prepDTO.Workspaces[0].ID
		}
		branches, _ := h.ListConversionBranches.Execute(r.Context(), targetWS)

		view := state.ConvertIssueView{
			Preparation: prepDTO,
			Branches:    dto.BranchesFromDomain(branches),
			Error:       err.Error(),
		}
		web.Render(w, r, http.StatusOK, pages.JiraConvertPage(view))
		return
	}

	convertedDTO := dto.ConvertedFromDomain(converted)
	prep, pErr := h.PrepareConversion.Execute(r.Context(), key)
	var prepDTO dto.ConversionPreparationDTO
	if pErr == nil {
		prepDTO = dto.PreparationFromDomain(prep)
	} else {
		prepDTO = dto.ConversionPreparationDTO{Issue: dto.IssueDTO{Key: key}}
	}

	view := state.ConvertIssueView{
		Preparation: prepDTO,
		Converted:   &convertedDTO,
	}
	web.Render(w, r, http.StatusOK, pages.JiraConvertPage(view))
}
