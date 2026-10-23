package dto

import "strings"

// modelValueSeparator joins provider and model inside the value of a model option. U+0001 cannot
// appear in a provider or model id, so the split is unambiguous.
const modelValueSeparator = "\u0001"

// EncodeModelValue encodes (provider, model) as the single value of an <option>.
func EncodeModelValue(provider, model string) string {
	return provider + modelValueSeparator + model
}

// DecodeModelValue reverses EncodeModelValue. It returns false for a value without the separator
// or with an empty part, which the bar treats as "default model".
func DecodeModelValue(value string) (provider, model string, ok bool) {
	provider, model, found := strings.Cut(value, modelValueSeparator)
	if !found || provider == "" || model == "" {
		return "", "", false
	}
	return provider, model, true
}
