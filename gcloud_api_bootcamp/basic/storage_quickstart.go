package main

import(
       "context"
       "fmt"
       "log"
       "time"

       "cloud.google.com/go/storage"
)

func main() {
    ctx := context.Background()

    // Sets the Google Cloud Platform Project ID
    projectID := "gpchandson-api"

    // Creates a client
    client, err := storage.NewClient(ctx)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }

    defer client.Close()

    bucketName := "my-new-bucket-20-11-2025"

    // Create bucket instanace
    bucket := client.Bucket(bucketName)

    // Creates the new bucket
    ctx, cancel := context.WithTimeout(ctx, time.Second * 10)
    defer cancel()

    if err := bucket.Create(ctx, projectID, nil); err != nil {
        log.Fatalf("Failed to create bucket: %v", err)    
    }

    fmt.Printf("Bucket %v created. \n", bucketName)
}
