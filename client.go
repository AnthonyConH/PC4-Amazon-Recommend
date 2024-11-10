package main

import (
    "encoding/gob"
    "fmt"
    "net"
    "os"
    "strings"
)

type UserData struct {
    PurchasedProducts []UserProduct
    Recommendations   []Recommendation
}

type UserProduct struct {
    ProductID string
    Rating    float64
    Category  string
}

type Recommendation struct {
    ProductID string
    Category  string
}

func main() {
    // Obtener el ID de usuario desde los argumentos o la entrada estándar
    var userID string
    if len(os.Args) > 1 {
        userID = os.Args[1]
    } else {
        fmt.Print("Ingrese el ID de usuario: ")
        fmt.Scanln(&userID)
        userID = strings.TrimSpace(userID)
    }

    // Conectarse al servidor
    conn, err := net.Dial("tcp", "localhost:9001")
    if err != nil {
        fmt.Println("Error al conectarse al servidor:", err)
        return
    }
    defer conn.Close()

    encoder := gob.NewEncoder(conn)
    decoder := gob.NewDecoder(conn)

    // Enviar el ID de usuario al servidor
    err = encoder.Encode(userID)
    if err != nil {
        fmt.Println("Error al enviar el ID de usuario al servidor:", err)
        return
    }

    // Recibir los datos del servidor
    var userData UserData
    err = decoder.Decode(&userData)
    if err != nil {
        fmt.Println("Error al recibir los datos del servidor:", err)
        return
    }

    // Mostrar los productos comprados por el usuario
    fmt.Printf("Productos comprados por el usuario %s:\n", userID)
    if len(userData.PurchasedProducts) == 0 {
        fmt.Println("\tNo se encontraron productos comprados para este usuario.")
    } else {
        for _, product := range userData.PurchasedProducts {
            fmt.Printf("\tProducto: %s, Calificación: %.2f, Categoría: %s\n", product.ProductID, product.Rating, product.Category)
        }
    }

    // Mostrar las recomendaciones
    fmt.Printf("\nRecomendaciones para el usuario %s:\n", userID)
    if len(userData.Recommendations) == 0 {
        fmt.Println("\tNo hay suficientes datos para generar recomendaciones.")
    } else {
        for _, rec := range userData.Recommendations {
            fmt.Printf("\tProducto: %s, Categoría: %s\n", rec.ProductID, rec.Category)
        }
    }
}
