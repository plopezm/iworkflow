package components

func treeChevronClass(open bool) string {
	if open {
		return "tree-chevron tree-chevron-open"
	}
	return "tree-chevron"
}

// folderPath is the outline of a folder icon, open or closed.
func folderPath(open bool) string {
	if open {
		return "M2 5.5A1.5 1.5 0 013.5 4h2.6l1.3 1.5h5.1A1.5 1.5 0 0114 7v4.5a1.5 1.5 0 01-1.5 1.5h-9A1.5 1.5 0 012 11.5v-6z"
	}
	return "M2 4.5A1.5 1.5 0 013.5 3h2.6l1.3 1.5h5.1A1.5 1.5 0 0114 6v5.5a1.5 1.5 0 01-1.5 1.5h-9A1.5 1.5 0 012 11.5v-7z"
}
