package components

import "encoding/json"

// pickerVals is the hx-vals of a picker request: the folder to list and the filter to apply.
func pickerVals(dir string, filter string) string {
	return jsonVals(map[string]string{"dir": dir, "filter": filter})
}

// dirVals is the hx-vals of the filter request: only the folder, so the text typed in the filter is not overridden.
func dirVals(dir string) string {
	return jsonVals(map[string]string{"dir": dir})
}

// chooseVals is the hx-vals of the request that picks a file.
func chooseVals(file string) string {
	return jsonVals(map[string]string{"file": file})
}

func jsonVals(values map[string]string) string {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}" // a map of strings always encodes; kept to never emit an invalid attribute
	}
	return string(encoded)
}
