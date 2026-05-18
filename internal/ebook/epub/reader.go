package epub

import (
    "faun.projects/margaret/margaret-ebook-library/internal/ebook"
)

func Read(file string) (*ebook.Metadata, error) {
    metadata, err := ebook.ExtractMetadata(file)
    if err != nil {
        return nil, err
    }
    // Additional logic to read the ebook content
    return metadata, nil
}
