package inbound

import (
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
)

// toNetworkAccessDTO maps the domain status to the card data: one access link and QR code per
// interface, and the token string only while it is in play.
func toNetworkAccessDTO(status models.NetworkAccessStatus) dto.NetworkAccessDTO {
	var token *string
	if status.Token != nil {
		value := status.Token.Value()
		token = &value
	}
	return dto.NewNetworkAccessDTO(status.Enabled, status.Active, status.LockedByEnv, status.Port).
		WithTokenState(status.RequireToken, status.TokenActive).
		WithConfiguredPort(status.ConfiguredPort).
		WithTargets(toAccessTargets(status)).
		WithToken(token)
}

// toAccessTargets builds the link and QR of every interface. The links carry the token while it is
// in play, so a scanned code signs the phone in.
func toAccessTargets(status models.NetworkAccessStatus) []dto.AccessTargetDTO {
	targets := make([]dto.AccessTargetDTO, 0, len(status.Targets))
	for _, network := range status.Targets {
		url := BuildAccessURL(network.Address, status.Port, status.Token)
		target := dto.AccessTargetDTO{
			Interface: network.Name,
			Address:   network.Address,
			AccessURL: url,
		}
		if svg, ok := RenderQRSVG(url); ok {
			target.QRSVG = &svg
		}
		if status.TLSPort != 0 {
			target.SecureURL = BuildSchemeURL("https", network.Address, status.TLSPort, status.Token)
			if svg, ok := RenderQRSVG(target.SecureURL); ok {
				target.SecureQRSVG = &svg
			}
		}
		targets = append(targets, target)
	}
	return targets
}
