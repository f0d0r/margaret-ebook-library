package main
                                                                                
import (                                                                       
    "fmt"                                                                      
    "faun.projects/margaret/margaret-ebook-library/pkg/ebook/epub"             
)                                                                              
                                                                               
func main() {                                                                  
    book, err := epub.Read("example.epub")                                     
    if err != nil {                                                            
        fmt.Println("Error reading ebook:", err)                               
        return                                                                 
    }                                                                          
    fmt.Printf("Book Title: %s\n", book.Title)                                 
} 