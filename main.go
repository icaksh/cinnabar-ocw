package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const(
	usernameEncoded = "" // Email Base64Encoded
	passwordEncoded = "" // Password Base64Encoded
)

type App struct{
	Client *http.Client
}

type AuthState struct{
	AuthState string
}

type SAMLResponse struct{
	SAMLResponse string
}

type Data struct{
	Status string
	WisPresensi []WisPresensi `json:"wisPresensi"`
	DurungPresensi []DurungPresensi `json:"durungPresensi"`
}

type DurungPresensi struct{
	NamaMatkul string `json:"namaMatkul"`
	WaktuMatkul string `json:"waktuMatkul"`
	NamaDosen string `json:"namaDosen"`
	LinkPresensi string `json:"linkPresensi"`
}

type WisPresensi struct{
	NamaMatkul string `json:"namaMatkul"`
	WaktuMatkul string `json:"waktuMatkul"`
	NamaDosen string `json:"namaDosen"`
}

func (app *App) getToken() AuthState{
	loginURL := "https://ocw.uns.ac.id/saml/login"
	client := app.Client

	response, err := client.Get(loginURL)

	if err != nil {
		log.Fatalln("Gagal mengambil response. ", err)
	}
	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}
	
	token, _ := document.Find("input[name='AuthState']").Attr("value")
	authState := AuthState{
		AuthState: token,
	}
	
	return authState
}

func (app *App) getSAMLResponse() SAMLResponse{
	client := app.Client

	authState := app.getToken()
	loginUrl := "https://sso.uns.ac.id/module.php/core/loginuserpass.php"
	usernameEd, _ := base64.StdEncoding.DecodeString(usernameEncoded)
	username := string(usernameEd)
	passwordEd, _ := base64.StdEncoding.DecodeString(passwordEncoded)
    password := string(passwordEd)
	data := url.Values{
		"AuthState": {authState.AuthState},
		"username" : {username},
		"password" : {password},
	}

	response, err := client.PostForm(loginUrl, data)

	if err != nil{
		log.Fatalln(err)
	}

	defer response.Body.Close()
	
	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}
	token, _ := document.Find("input[name='SAMLResponse']").Attr("value")
	
	saml := SAMLResponse{
		SAMLResponse: token,
	}
	
	return saml
}


func (app *App) login() {
	client := app.Client

	SAMLResponse := app.getSAMLResponse()
	loginUrl := "https://ocw.uns.ac.id/saml/acs"
	data := url.Values{
		"SAMLResponse": {SAMLResponse.SAMLResponse},
	}

	response, err := client.PostForm(loginUrl, data)

	if err != nil{
		log.Fatalln(err)
	}

	defer response.Body.Close()

	_, err = ioutil.ReadAll(response.Body)
	
	if err != nil {
		log.Fatalln(err)
	}
} 

func (app *App) getMatkul() Data{
	presensiUrl := "https://ocw.uns.ac.id/presensi-online-mahasiswa/kuliah-berlangsung"
	client := app.Client

	response, err := client.Get(presensiUrl)

	if err != nil {
		log.Fatalln("Gagal mengambil response. ", err)
	}
	defer response.Body.Close()

	document, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body. ", err)
	}

	var rungPresensi []DurungPresensi
	var wisPresensi []WisPresensi

	document.Find("div[style]").Each(func(i int, s *goquery.Selection) {
		namaMatkul := strings.TrimSpace(s.Find("p").First().Text())
		waktuMatkul := strings.TrimSpace(s.Find("small").First().Text())
		namaDosen := strings.TrimSpace(s.Find("small").Last().Text())
		linkMatkul, ok := s.Find("a").Attr("href")
		if(ok){
			presensiUrl := "https://ocw.uns.ac.id"+linkMatkul
			client := app.Client

			response, err := client.Get(presensiUrl)

			if err != nil {
				log.Fatalln("Gagal mengambil response. ", err)
			}
			defer response.Body.Close()

			document, err := goquery.NewDocumentFromReader(response.Body)
			if err != nil {
				log.Fatal("Error loading HTTP response body. ", err)
			}

			linkPresensi,_ := document.Find("div[style] a").First().Attr("href")

			durung := DurungPresensi{
				NamaMatkul: namaMatkul,
				WaktuMatkul: waktuMatkul,
				NamaDosen: namaDosen,
				LinkPresensi: "https://ocw.uns.ac.id"+linkPresensi,
			}
			rungPresensi = append(rungPresensi, durung)
		}else{
			uwis := WisPresensi{
				NamaMatkul: namaMatkul,
				WaktuMatkul: waktuMatkul,
				NamaDosen: namaDosen,
			}
			wisPresensi = append(wisPresensi, uwis)
		}

	})
	data := Data{
		Status: "success",
		WisPresensi: wisPresensi,
		DurungPresensi: rungPresensi,
	}
	
	return data
}

func presensi(w http.ResponseWriter, r *http.Request) {
	jar, _ := cookiejar.New(nil)

	app := App{
		Client: &http.Client{Jar: jar},
	}

	app.login()
	data := app.getMatkul()
	
    w.Header().Set("Content-Type", "application/json")

    if r.Method == "POST" {
        result, err := json.Marshal(data)

        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        w.Write(result)
        return
    }

    http.Error(w, "404 page not found", http.StatusNotFound)
}

func main() {
	http.HandleFunc("/presensi", presensi)
	fmt.Println("APInya Jalan (Hayasaka)")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
    http.ListenAndServe(":"+port, nil)
}