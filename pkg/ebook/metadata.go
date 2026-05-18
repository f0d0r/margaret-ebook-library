package ebook

type Metadata struct {
    Title   string
    Author  string
    ISBN    string
}

func ExtractMetadata(file string) (*Metadata, error) {
    // Implementation to extract metadata from the file
    return &Metadata{}, nil
}
