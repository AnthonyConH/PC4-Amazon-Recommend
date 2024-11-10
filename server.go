package main

import (
    "encoding/gob"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "sort"
    "sync"
)

type RatingsData map[string]map[string]float64   // usuario -> producto -> calificación
type ProductCategories map[string]string        // producto -> categoría
type CategoryProducts map[string][]string       // categoría -> lista de productos

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

func loadRatingsData(filename string) (RatingsData, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    decoder := json.NewDecoder(file)
    var data RatingsData
    err = decoder.Decode(&data)
    return data, err
}

func loadProductCategories(filename string) (ProductCategories, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    decoder := json.NewDecoder(file)
    var data ProductCategories
    err = decoder.Decode(&data)
    return data, err
}

func main() {
    // Cargar los datos de calificaciones y categorías
    ratings, err := loadRatingsData("ratings.json")
    if err != nil {
        fmt.Println("Error al cargar los datos de calificaciones:", err)
        return
    }

    categories, err := loadProductCategories("categories.json")
    if err != nil {
        fmt.Println("Error al cargar las categorías de productos:", err)
        return
    }

    categoryProducts, productPopularity := createCategoryProductsMap(ratings, categories)

    // Configurar el servidor TCP
    listener, err := net.Listen("tcp", ":9001")
    if err != nil {
        fmt.Println("Error al iniciar el servidor:", err)
        return
    }
    fmt.Println("Servidor escuchando en el puerto 9001...")

    var wg sync.WaitGroup

    // Aceptar conexiones de clientes de manera continua
    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Println("Error al aceptar conexión:", err)
            continue
        }
        fmt.Println("Cliente conectado:", conn.RemoteAddr())
        wg.Add(1)
        go handleClient(conn, ratings, categories, categoryProducts, productPopularity, &wg)
    }
}

func handleClient(conn net.Conn, ratings RatingsData, categories ProductCategories, categoryProducts CategoryProducts, productPopularity map[string]int, wg *sync.WaitGroup) {
    defer wg.Done()
    defer conn.Close()

    encoder := gob.NewEncoder(conn)
    decoder := gob.NewDecoder(conn)

    // Recibir el ID de usuario del cliente
    var userID string
    err := decoder.Decode(&userID)
    if err != nil {
        fmt.Println("Error al recibir el ID de usuario del cliente:", err)
        return
    }

    fmt.Printf("Procesando datos para el usuario '%s'\n", userID)

    // Verificar que el usuario exista y obtener los productos que compró
    userProducts, exists := ratings[userID]
    if !exists {
        fmt.Printf("Usuario '%s' no encontrado.\n", userID)
        // Enviar una respuesta vacía
        userData := UserData{
            PurchasedProducts: []UserProduct{},
            Recommendations:   []Recommendation{},
        }
        encoder.Encode(userData)
        return
    }

    // Preparar la lista de productos comprados por el usuario
    var purchasedProducts []UserProduct
    for productID, rating := range userProducts {
        category, exists := categories[productID]
        if !exists {
            category = "Desconocida"
        }
        purchasedProducts = append(purchasedProducts, UserProduct{
            ProductID: productID,
            Rating:    rating,
            Category:  category,
        })
    }

    // Calcular las preferencias del usuario
    categoryCounts := calculateUserCategoryPreferences(userProducts, categories)
    preferredCategories := getUserPreferredCategories(categoryCounts)
    if len(preferredCategories) == 0 {
        // Enviar los productos comprados sin recomendaciones
        userData := UserData{
            PurchasedProducts: purchasedProducts,
            Recommendations:   []Recommendation{},
        }
        encoder.Encode(userData)
        return
    }

    // Generar recomendaciones
    recs := recommendProducts(userProducts, categories, categoryProducts, productPopularity, preferredCategories, 5)

    // Preparar los datos para enviar al cliente
    userData := UserData{
        PurchasedProducts: purchasedProducts,
        Recommendations:   recs,
    }

    // Enviar los datos al cliente
    err = encoder.Encode(userData)
    if err != nil {
        fmt.Println("Error al enviar los datos al cliente:", err)
        return
    }

    fmt.Println("Datos enviados al cliente:", conn.RemoteAddr())
}

// Funciones auxiliares
func createCategoryProductsMap(ratings RatingsData, categories ProductCategories) (CategoryProducts, map[string]int) {
    categoryProducts := make(CategoryProducts)
    productPopularity := make(map[string]int) // producto -> número de compras

    for _, products := range ratings {
        for product := range products {
            category, exists := categories[product]
            if !exists {
                continue // Si el producto no tiene categoría, lo omitimos
            }
            categoryProducts[category] = append(categoryProducts[category], product)
            productPopularity[product]++
        }
    }

    return categoryProducts, productPopularity
}

func calculateUserCategoryPreferences(userProducts map[string]float64, categories ProductCategories) map[string]int {
    categoryCounts := make(map[string]int) // categoría -> número de productos

    for product := range userProducts {
        category, exists := categories[product]
        if !exists {
            continue // Si el producto no tiene categoría, lo omitimos
        }
        categoryCounts[category]++
    }

    return categoryCounts
}

func getUserPreferredCategories(categoryCounts map[string]int) []string {
    maxCount := 0
    for _, count := range categoryCounts {
        if count > maxCount {
            maxCount = count
        }
    }

    // Obtener todas las categorías con el conteo máximo
    preferredCategories := []string{}
    for category, count := range categoryCounts {
        if count == maxCount {
            preferredCategories = append(preferredCategories, category)
        }
    }

    return preferredCategories
}

func recommendProducts(userProducts map[string]float64, categories ProductCategories, categoryProducts CategoryProducts, productPopularity map[string]int, preferredCategories []string, numRecommendations int) []Recommendation {
    // Crear un mapa para evitar duplicados y excluir los productos que el usuario ya compró
    productSet := make(map[string]bool)
    for product := range userProducts {
        productSet[product] = true
    }

    type ProductInfo struct {
        ProductID  string
        Popularity int
        Category   string
    }
    var products []ProductInfo

    for _, category := range preferredCategories {
        productsInCategory := categoryProducts[category]
        for _, product := range productsInCategory {
            if productSet[product] {
                continue // Excluir productos ya comprados por el usuario
            }
            if _, exists := productSet[product]; exists {
                continue // Evitar duplicados
            }
            popularity := productPopularity[product]
            products = append(products, ProductInfo{ProductID: product, Popularity: popularity, Category: category})
            productSet[product] = true
        }
    }

    // Ordenar los productos por popularidad descendente
    sort.Slice(products, func(i, j int) bool {
        return products[i].Popularity > products[j].Popularity
    })

    // Generar la lista de recomendaciones
    recommendations := []Recommendation{}
    for _, productInfo := range products {
        recommendations = append(recommendations, Recommendation{
            ProductID: productInfo.ProductID,
            Category:  productInfo.Category,
        })
        if len(recommendations) >= numRecommendations {
            break
        }
    }

    return recommendations
}
