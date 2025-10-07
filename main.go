package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)



type Pasien struct {
    ID              string
    Nama            string
    Usia            int
    JenisKelamin    string
    TanggalLahir    sql.NullTime   
    Alamat          string         
    RiwayatPenyakit sql.NullString 
    Telepon         string         
}

type Transaksi struct {
	ID          string
	Deskripsi   string
	TotalBiaya  float64
}

type DetailPasienData struct {
	PasienData    Pasien
	TransaksiData []Transaksi
}

type DashboardData struct {
	TotalPasien    int
	TotalTransaksi int
	TotalBiaya     float64
}


var db *sql.DB
var templates map[string]*template.Template


func init() {
	dsn := "root:@tcp(127.0.0.1:3306)/rs_db?parseTime=true"
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Gagal koneksi database:", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Database tidak bisa diakses:", err)
	}
	fmt.Println(" Berhasil terhubung ke database MySQL!")


	loadTemplates()
}

func loadTemplates() {
	templates = make(map[string]*template.Template)


	pages, err := filepath.Glob("templates/*.html")
	if err != nil {
		log.Fatal("Gagal mencari file template:", err)
	}
	

	layouts, err := filepath.Glob("templates/layout.html")
	if err != nil {
		log.Fatal("Gagal mencari file layout:", err)
	}


	for _, page := range pages {
		if filepath.Base(page) == "layout.html" || filepath.Base(page) == "login.html" {
			continue
		}


		files := append(layouts, page)
		
		name := filepath.Base(page)
		
		tmpl, err := template.New(name).ParseFiles(files...)
		if err != nil {
			log.Fatalf("Gagal parse template %s: %v", name, err)
		}
		templates[name] = tmpl
	}
	

	tmplLogin, err := template.ParseFiles("templates/login.html")
    if err != nil {
        log.Fatal("Gagal parse login.html:", err)
    }
    templates["login.html"] = tmplLogin


	fmt.Println(" Semua template berhasil di-load!")
}

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmpl, ok := templates[tmplName]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s tidak ditemukan!", tmplName), http.StatusInternalServerError)
		return
	}
	

	err := tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
	
		err = tmpl.Execute(w, data)
		if err != nil {
			log.Printf("Gagal render template %s: %v", tmplName, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}


func main() {
	defer db.Close() 

	// Routing
	http.HandleFunc("/", authMiddleware(homeHandler))
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/registrasi-pasien", authMiddleware(registrasiHandler))
	http.HandleFunc("/pasien", authMiddleware(detailHandler))
	http.HandleFunc("/dashboard", authMiddleware(dashboardHandler))
	http.HandleFunc("/edit-pasien", authMiddleware(editPasienHandler))
	http.HandleFunc("/hapus-pasien", authMiddleware(hapusPasienHandler))
	http.HandleFunc("/tambah-transaksi", authMiddleware(tambahTransaksiHandler))

	port := ":8080"
	fmt.Printf(" Server berjalan di http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}



func homeHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, nama, usia, jenis_kelamin, tanggal_lahir, alamat, riwayat_penyakit, telepon FROM pasien ORDER BY nama ASC")
	if err != nil {
		http.Error(w, "Gagal ambil data pasien", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarPasien []Pasien
	for rows.Next() {
		var p Pasien
		if err := rows.Scan(&p.ID, &p.Nama, &p.Usia, &p.JenisKelamin, &p.TanggalLahir, &p.Alamat, &p.RiwayatPenyakit, &p.Telepon); err != nil {
			log.Println("Error scanning pasien:", err) 
			continue
		}
		daftarPasien = append(daftarPasien, p)
	}

	renderTemplate(w, "index.html", daftarPasien)
}

func registrasiHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		renderTemplate(w, "registrasi.html", nil)
	case http.MethodPost:
		r.ParseForm()
		usia, err := strconv.Atoi(r.FormValue("usia"))
		if err != nil {
			http.Error(w, "Input usia tidak valid", http.StatusBadRequest)
			return
		}

		_, err = db.Exec(`
			INSERT INTO pasien (id, nama, usia, jenis_kelamin, tanggal_lahir, alamat, riwayat_penyakit, telepon)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r.FormValue("id_pasien"),
			r.FormValue("nama_pasien"),
			usia,
			r.FormValue("jenis_kelamin"),
			r.FormValue("tanggal_lahir"),
			r.FormValue("alamat"),
			r.FormValue("riwayat_penyakit"),
			r.FormValue("telepon"),
		)
		if err != nil {
			http.Error(w, "Gagal simpan data pasien: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}

func detailHandler(w http.ResponseWriter, r *http.Request) {
	pasienID := r.URL.Query().Get("id")
	if pasienID == "" {
		http.Error(w, "ID pasien wajib diisi", http.StatusBadRequest)
		return
	}

	var pasien Pasien
	err := db.QueryRow(
		"SELECT id, nama, usia, jenis_kelamin, tanggal_lahir, alamat, riwayat_penyakit, telepon FROM pasien WHERE id = ?",
		pasienID,
	).Scan(&pasien.ID, &pasien.Nama, &pasien.Usia, &pasien.JenisKelamin, &pasien.TanggalLahir, &pasien.Alamat, &pasien.RiwayatPenyakit, &pasien.Telepon)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Pasien tidak ditemukan", http.StatusNotFound)
		} else {
			http.Error(w, "Error query detail pasien: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	rows, err := db.Query("SELECT id, deskripsi, total_biaya FROM transaksi WHERE pasien_id = ?", pasienID)
	if err != nil {
		http.Error(w, "Gagal ambil data transaksi", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var daftarTransaksi []Transaksi
	for rows.Next() {
		var t Transaksi
		if err := rows.Scan(&t.ID, &t.Deskripsi, &t.TotalBiaya); err != nil {
			log.Println("Error scanning transaksi:", err)
			continue
		}
		daftarTransaksi = append(daftarTransaksi, t)
	}

	data := DetailPasienData{PasienData: pasien, TransaksiData: daftarTransaksi}
	renderTemplate(w, "detail.html", data)
}

func tambahTransaksiHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()
	pasienID := r.FormValue("pasien_id")
	deskripsi := r.FormValue("deskripsi")
	totalBiayaStr := r.FormValue("total_biaya")

	totalBiaya, err := strconv.ParseFloat(totalBiayaStr, 64)
	if err != nil {
		http.Error(w, "Input biaya tidak valid", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(`
		INSERT INTO transaksi (pasien_id, deskripsi, total_biaya, tanggal)
		VALUES (?, ?, ?, NOW())`,
		pasienID, deskripsi, totalBiaya,
	)
	if err != nil {
		http.Error(w, "Gagal menyimpan transaksi: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/pasien?id="+pasienID, http.StatusSeeOther)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		renderTemplate(w, "login.html", nil)
	case "POST":
		r.ParseForm()
		if r.FormValue("username") == "admin" && r.FormValue("password") == "admin123" {
			http.SetCookie(w, &http.Cookie{
				Name:    "session_token",
				Value:   "logged-in",
				Expires: time.Now().Add(1 * time.Hour),
				Path:    "/",
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
		
			renderTemplate(w, "login.html", "Login gagal! Username/password salah")
		}
	default:
		http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Now().Add(-1 * time.Hour),
		Path:    "/",
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "logged-in" {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func editPasienHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		id := r.URL.Query().Get("id")
		var p Pasien
		err := db.QueryRow("SELECT id, nama, usia, jenis_kelamin, tanggal_lahir, alamat, riwayat_penyakit, telepon FROM pasien WHERE id = ?", id).
			Scan(&p.ID, &p.Nama, &p.Usia, &p.JenisKelamin, &p.TanggalLahir, &p.Alamat, &p.RiwayatPenyakit, &p.Telepon)
		if err != nil {
			http.Error(w, "Pasien tidak ditemukan", http.StatusNotFound)
			return
		}
		renderTemplate(w, "edit.html", p)
	case "POST":
		r.ParseForm()
		usia, err := strconv.Atoi(r.FormValue("usia"))
		if err != nil {
			http.Error(w, "Input usia tidak valid", http.StatusBadRequest)
			return
		}
		
		_, err = db.Exec(`
			UPDATE pasien 
			SET nama=?, usia=?, jenis_kelamin=?, tanggal_lahir=?, alamat=?, riwayat_penyakit=?, telepon=? 
			WHERE id=?`,
			r.FormValue("nama_pasien"),
			usia,
			r.FormValue("jenis_kelamin"),
			r.FormValue("tanggal_lahir"),
			r.FormValue("alamat"),
			r.FormValue("riwayat_penyakit"),
			r.FormValue("telepon"),
			r.FormValue("id"),
		)
		if err != nil {
			http.Error(w, "Gagal update data pasien", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func hapusPasienHandler(w http.ResponseWriter, r *http.Request) {
 
    if r.Method != http.MethodDelete {
        http.Error(w, "Metode tidak diizinkan", http.StatusMethodNotAllowed)
        return
    }

   
    id := r.URL.Query().Get("id")
    if id == "" {
        http.Error(w, "ID Pasien tidak boleh kosong", http.StatusBadRequest)
        return
    }

   
    _, err := db.Exec("DELETE FROM transaksi WHERE pasien_id = ?", id)
    if err != nil {
        http.Error(w, "Gagal hapus transaksi terkait pasien: "+err.Error(), http.StatusInternalServerError)
        return
    }

    _, err = db.Exec("DELETE FROM pasien WHERE id = ?", id)
    if err != nil {
        http.Error(w, "Gagal hapus pasien: "+err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}


func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	var data DashboardData
	
	err := db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM pasien),
			(SELECT COUNT(*) FROM transaksi),
			(SELECT IFNULL(SUM(total_biaya), 0) FROM transaksi)
	`).Scan(&data.TotalPasien, &data.TotalTransaksi, &data.TotalBiaya)
	
	if err != nil {
		log.Println("!!! ERROR QUERY DASHBOARD:", err)
		http.Error(w, "Gagal mengambil data dashboard", http.StatusInternalServerError)
		return
	}

	renderTemplate(w, "dashboard.html", data)
}