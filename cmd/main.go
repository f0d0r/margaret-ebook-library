package main

import (
	"fmt"

	"faun.projects/margaret/margaret-ebook-library/pkg/ebook"
)                                                                              
                                                                               
func main() {                                                                  
    bookMeta, err := ebook.ReadMetadata("example.epub")                                     
    if err != nil {                                                            
        fmt.Println("Error reading ebook:", err)                               
        return                                                                 
    }                                                                          
    fmt.Printf("Book Title: %s\n", bookMeta.Title)                                 
} 