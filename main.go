package main

import (
    "context"
     flags "github.com/jessevdk/go-flags"
    "fmt"
    "html/template"
    "log"
    "math/rand"
    "net"
    "net/http"
    "os"
    "sort"
    "strings"
    "time"

    "github.com/gorilla/mux"
    "github.com/mpolden/echoip/iputil/geo"

    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

func helloHandler(w http.ResponseWriter, req *http.Request) {
    var data = struct {
    }{
    }

    t, _ := template.ParseFiles("index.html")
    t.Execute(w, &data)
}

type Newsmth struct {
    Id string `json:"id"`
    Ip string `json:"ip"`
    Date time.Time `json:"date"`
    Board string `json:"board"`
}

var gr, _ = geo.Open("data/country.mmdb", "data/city.mmdb", "data/asn.mmdb")

var _ids []string
var _ips []string
var _boards []string
var _id_cities  map[string][]string = make(map[string][]string) 
var _city_points map[string][]uint32 = make(map[string][]uint32)

type Vertex struct {
	Lon, Lat float64
}

var _ip_lon_lat map[string]Vertex = make(map[string]Vertex)

func addIp(ip string) {
    for _, _ip := range _ips {
        if _ip == ip {
            return
        }
    }

    city, _ := gr.City(net.ParseIP(ip))

    _rand_lon := rand.Float64()/100
    _rand_lat := rand.Float64()/100

    _ip_lon_lat[ip] = Vertex{
        city.Longitude + _rand_lon, city.Latitude + _rand_lat,
	}

    _ips = append(_ips, ip)

}

func addId(id string) {
    for _, _id := range _ids {
        if _id == id {
            return
        }
    }

    _ids = append(_ids, id)
}
func addBoard(board string) {
    for _, _board := range _boards {
        if _board == board {
            return
        }
    }

    _boards = append(_boards, board)
}

func addSmth(smths map[string][]Newsmth, smth Newsmth) {
    city1, _ := gr.City(net.ParseIP(smth.Ip))

    if len(smths[smth.Id]) == 0 {
        smths[smth.Id] = []Newsmth{smth}
        addId(smth.Id)
        addIp(smth.Ip)
        addBoard(smth.Board)

        _id_cities[smth.Id] = append(_id_cities[smth.Id], city1.Name)
    } else {

        for _, _smth := range smths[smth.Id] {
            // if smth.Ip == _smth.Ip && smth.Board == _smth.Board {
            if smth.Ip == _smth.Ip {
                return
            }

            city2, _ := gr.City(net.ParseIP(_smth.Ip))

            if city2.Name != "" && city1.Name == city2.Name {
                return
            }

            _id_cities[smth.Id] = append(_id_cities[smth.Id], city2.Name)
        }

        smths[smth.Id] = append(smths[smth.Id], smth)

        addIp(smth.Ip)
        addBoard(smth.Board)
    }
}

// pull the data from DB and create newsmth.js 
func createNewsmthJS() {
    ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
    client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
    collection := client.Database("smthmapper").Collection("newsmth")

    ctx, _ = context.WithTimeout(context.Background(), 2000*time.Second)
    cur, err := collection.Find(ctx, bson.D{})
    if err != nil { log.Fatal(err) }
    defer cur.Close(ctx)

    var smths map[string][]Newsmth = make(map[string][]Newsmth)

    var cnt uint32 = 0

    for cur.Next(ctx) {
       var smth Newsmth
       err := cur.Decode(&smth)
       if err != nil { log.Fatal(err) }
            // {id: 'aaa', lon: '-0.1279688', lat: '51.5077286'}

       if smth.Ip == "127.0.0.1" {
            continue
       }

       // smths = append(smths, smth)
       addSmth(smths, smth)

       cnt = cnt + 1

       if  cnt % 10000 == 0 {
            fmt.Printf("Count = %v\n", cnt)
       }
    }

    fmt.Printf("Total = %v", cnt)

    /*
    for id, _smths := range smths {
        fmt.Printf("Len: %v %v\n", id, len(_smths))
    }
    */

    sort.Sort(sort.StringSlice(_ids))
    sort.Sort(sort.StringSlice(_ips))
    sort.Sort(sort.StringSlice(_boards))

    f_newsmth, err := os.Create("assets/newsmth.js")
    if err != nil { log.Fatal(err) }

    // write out IDs
    f_newsmth.WriteString("ids = [\n")

    for _, _id := range _ids{
        p := fmt.Sprintf("'%v',\n", _id)
        f_newsmth.WriteString(p)
    }

    f_newsmth.WriteString("];\n")

    // write out ip addresses
    f_newsmth.WriteString("ips = [\n")

    for _, _ip := range _ips{
        p := fmt.Sprintf("'%v',\n", _ip)
        f_newsmth.WriteString(p)
    }

    f_newsmth.WriteString("];\n")

    // write out longitute lattitute
    // write out IDs
    f_newsmth.WriteString("ip_lon_lats = {\n")

    for ip, vetex :=  range _ip_lon_lat {
        idx_ip := sort.StringSlice(_ips).Search(ip)
        p := fmt.Sprintf("%v: [%v, %v],\n", idx_ip, vetex.Lon, vetex.Lat)
        f_newsmth.WriteString(p)
    }

    f_newsmth.WriteString("};\n")

    f_newsmth.WriteString("// idx_id, idx_board, idx_ip\n")
    p := fmt.Sprintf("smth_points = [\n")
    f_newsmth.WriteString(p)

    // write out records
    var idx_smth  uint32 = 0
    for _, _smths := range smths {
        for _, smth := range _smths {
            idx_id := sort.StringSlice(_ids).Search(smth.Id)
            idx_ip := sort.StringSlice(_ips).Search(smth.Ip)
            idx_board := sort.StringSlice(_boards).Search(smth.Board)

            // fmt.Println(lon_lat.Lon)
            // fmt.Println(lon_lat.Lat)

            //ip := net.ParseIP(smth.Ip)
            //city, _ := gr.City(ip)
            //fmt.Printf(city.Name)
            //fmt.Printf("Coordinates: %v, %v\n", city.Latitude, city.Longitude)

            //p := fmt.Sprintf("{id: '%s', ip: '%s', board: '%s', lon: '%v', lat: '%v'},\n", smth.Id, smth.Ip, smth.Board, city.Longitude, city.Latitude)
            //p := fmt.Sprintf("[%v, %v, %v, %v, %v],\n", idx_id, idx_board, idx_ip, idx_lon, idx_lat)
            p := fmt.Sprintf("[%v, %v, %v],\n", idx_id, idx_board, idx_ip )
            f_newsmth.WriteString(p)

            city, _ := gr.City(net.ParseIP(smth.Ip))

            //fmt.Println(city.Name)
            city_name := strings.ReplaceAll(city.Name, "'", "_")

            if len(_city_points[city_name]) == 0 {
                _city_points[city_name] = []uint32{idx_smth}
            } else {
                _city_points[city_name] = append(_city_points[city_name], idx_smth)
            }

            idx_smth++
        }
    }


    f_newsmth.WriteString("];")

    // write out records
    f2, err := os.Create("assets/newsmth_2.js")
    f2.WriteString("var newsmth = [\n")

    for _, _smths := range smths {
        for _, smth := range _smths {
            ip := net.ParseIP(smth.Ip)
            city, _ := gr.City(ip)

            p := fmt.Sprintf("{id: '%s', ip: '%s', board: '%s', lon: '%v', lat: '%v', date: '%v'},\n", smth.Id, smth.Ip, smth.Board, city.Longitude, city.Latitude, smth.Date)
            f2.WriteString(p)
        }
    }

    f2.WriteString("];")
}

func main() {
    var opts struct {
        Server bool     `short:"s" long:"server" description:"Run smth mapper server"`
        Create bool     `short:"c" long:"create" description:"Create newsmth.js for smth mapper"`
    }

    _, err := flags.ParseArgs(&opts, os.Args)

    if err != nil {
        os.Exit(1)
    }

    if opts.Create {
        createNewsmthJS()
        return
    }

    r := mux.NewRouter()
    staticFileDirectory := http.Dir("./assets/")
    staticFileHandler := http.StripPrefix("/assets/", http.FileServer(staticFileDirectory))
    r.PathPrefix("/assets/").Handler(staticFileHandler).Methods("GET")

    r.HandleFunc("/hello", helloHandler).Methods("GET")
    http.ListenAndServe(":8090", r)
}
