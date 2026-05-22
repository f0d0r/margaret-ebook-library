package epub

import "archive/zip"

// findFileInZip locates a file by path within a ZIP archive
func findFileInZip(zr *zip.Reader, path string) *zip.File {
	for _, file := range zr.File {
		if file.Name == path {
			return file
		}
	}
	return nil
}
