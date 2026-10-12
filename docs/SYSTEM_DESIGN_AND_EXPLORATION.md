# System Design & Deep Dive: Real-Time Monitoring & Event Streaming (GoTH Stack)

> **Stack:** Go (1.25) + Templ + HTMX + Tailwind CSS  
> **Tema:** Real-Time Infrastructure & Event Telemetry Dashboard ("PulseOps")  
> **Tujuan:** Menguasai arsitektur *Hypermedia-Driven Real-Time Applications* tanpa SPA framework.

---

## 1. Status Lingkungan: Go Terdeteksi!

Go telah ditemukan di sistem Anda pada lokasi:
- **Path Binary:** `C:\Program Files\Go\bin\go.exe`
- **Versi:** `go version go1.25.1 windows/amd64`

### Mengapa sebelumnya tidak terdeteksi?
Folder `C:\Program Files\Go\bin` belum terdaftar dalam Environment Variable `PATH` sistem Anda, atau sesi PowerShell belum membaca path tersebut.

### Cara Mengaktifkan ke PATH:
1. **Sementara (Hanya di sesi terminal aktif):**
   ```powershell
   $env:PATH += ";C:\Program Files\Go\bin"
   go version
   ```
2. **Permanen (Windows User / System PATH):**
   ```powershell
   [Environment]::SetEnvironmentVariable("Path", $env:Path + ";C:\Program Files\Go\bin", [EnvironmentVariableTarget]::User)
   ```
*(Catatan: Anda juga memiliki script profile `Microsoft.PowerShell_profile.ps1` yang memanggil `oh-my-posh` yang belum terpasang, namun ini tidak mengganggu eksekusi Go jika path-nya sudah disetel).*

---

## 2. Paradigma: SPA + WebSocket vs GoTH + SSE

Dalam pengembangan web real-time konvensional, arsitektur biasanya memisahkan frontend dan backend secara kaku:

```
[Arsitektur SPA Tradisional]
Server -> WebSocket (JSON) -> Client JS Parser -> State Store (Zustand/Redux) 
       -> React/Vue Virtual DOM Diffing -> DOM Patch
*Kelemahan: Kompleksitas ganda, duplikasi tipe data, bundle JS membengkak, rentan desync.*
```

Dengan **GoTH Stack + HTMX**, arsitekturnya bergeser menjadi **Hypermedia-Driven**:

```
[Arsitektur GoTH + HTMX]
Server Event Broker -> Render Templ Component ke HTML Fragment 
       -> SSE Stream (HTTP/1.1 or HTTP/2) -> HTMX SSE Extension 
       -> Direct DOM Swap + Out-of-Band (OOB) Updates
*Keunggulan: Zero client-side state store, single source of truth di server, type-safety murni di Go, sangat ringan.*
```

---

## 3. Anatomi Server-Sent Events (SSE) dengan HTMX

### 3.1 Mengapa SSE, bukan WebSockets?
- **Unidirectional (Server-to-Client):** Untuk dashboard monitoring, server adalah produser data dan browser adalah konsumer. SSE dirancang khusus untuk skenario ini.
- **Built-in Reconnection:** Protokol SSE (`EventSource`) memiliki mekanisme auto-reconnect bawaan browser dengan header `Last-Event-ID`. Jika koneksi terputus, browser otomatis mencoba menyambung kembali tanpa baris JS tambahan.
- **HTTP Standard:** Berjalan di atas HTTP biasa (port 80/443), ramah reverse-proxy (Nginx, Caddy, Cloudflare), dan bebas masalah firewall yang sering dihadapi WebSocket murni.

### 3.2 Format Wire SSE
Protokol SSE bekerja dengan format teks sederhana yang diakhiri oleh dua baris baru (`\n\n`):

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: telemetry-update
data: <div id="cpu-gauge" class="...">78%</div>
data: <div id="ram-gauge" class="...">4.2 GB</div>

event: new-log
data: <tr class="text-emerald-400"><td>[12:04:01]</td><td>Worker-3 healthy</td></tr>

```

### 3.3 Cara HTMX Menerima SSE (`hx-ext="sse"`)
HTMX menyediakan ekstensi resmi `sse.js`. Cara deklarasinya sangat deklaratif:

```html
<div hx-ext="sse" sse-connect="/api/v1/stream">
  
  <!-- Elemen ini mendengarkan event bertipe 'telemetry-update' -->
  <div sse-swap="telemetry-update" hx-swap="innerHTML">
    <!-- Konten awal sebelum streaming masuk -->
    <span>Menunggu sinyal server...</span>
  </div>

  <!-- Elemen ini mendengarkan event bertipe 'log-feed' -->
  <tbody sse-swap="log-feed" hx-swap="afterbegin">
    <!-- Log baru akan otomatis disisipkan di baris paling atas -->
  </tbody>

</div>
```

---

## 4. Senjata Rahasia: Out-of-Band (OOB) Swaps (`hx-swap-oob="true"`)

Inilah konsep terpenting yang membuat HTMX terasa seperti "sihir".

### 4.1 Problem
Dalam model swap standar, satu respons SSE hanya dapat memperbarui elemen yang menjadi targetnya (`sse-swap`). Namun di dashboard nyata:
> *Ketika ada 1 event server masuk (misal: "Database Spike Critical!"), kita ingin:*
> 1. Menambahkan baris log ke tabel audit log.
> 2. Mengubah indikator status server dari Hijau ke Merah.
> 3. Menambah angka counter unread alert di navbar (+1).
> 4. Memunculkan toast notification di pojok kanan bawah.

### 4.2 Solusi OOB HTMX
Tanpa perlu 4 koneksi SSE terpisah atau custom JavaScript event dispatcher, server Go mengirimkan **satu payload HTML** yang berisi elemen target utama PLUS elemen-elemen OOB yang memiliki atribut `hx-swap-oob="true"`.

#### Contoh Payload Server:
```html
<!-- 1. Elemen Utama (akan diswap ke container log) -->
<tr class="bg-red-950/40 border-red-800 text-red-300">
  <td>12:00:15</td>
  <td>CRITICAL</td>
  <td>CPU Core-0 throttled to 99%</td>
</tr>

<!-- 2. OOB Target: Indikator Status di Header -->
<div id="system-status-indicator" hx-swap-oob="outerHTML" class="flex items-center gap-2 text-rose-400 font-semibold">
  <span class="w-3 h-3 rounded-full bg-rose-500 animate-ping"></span>
  STATUS: CRITICAL
</div>

<!-- 3. OOB Target: Badge Counter Alert di Navbar -->
<span id="alert-badge" hx-swap-oob="innerHTML" class="bg-rose-600 text-white text-xs px-2 py-0.5 rounded-full font-bold">
  14 Alerts
</span>

<!-- 4. OOB Target: Pop-up Toast Notifikasi -->
<div id="toast-container" hx-swap-oob="afterbegin">
  <div class="p-3 mb-2 bg-slate-900 border border-rose-500 text-rose-200 rounded-lg shadow-xl shadow-rose-950/50 flex justify-between items-center transition-all animate-bounce">
    <span>⚠️ CPU Core-0 Spike Detected!</span>
    <button onclick="this.parentElement.remove()" class="text-xs text-slate-400 hover:text-white">✕</button>
  </div>
</div>
```

**Hasilnya:** HTMX akan memindai seluruh ID yang cocok di halaman browser (`#system-status-indicator`, `#alert-badge`, `#toast-container`) dan langsung menukarnya secara atomik.

---

## 5. Arsitektur Backend: Go + Templ

### 5.1 Pattern: SSE Event Broker (Fan-Out Pub/Sub)
Di Go, kita membuat sebuah `Broker` struct berbasis goroutines dan Go channels:

```
               +---------------------------------------+
               |          Background Telemetry         |
               |        (CPU Reader / Simulator)      |
               +---------------------------------------+
                                  |
                                  v
                      [ Events Channel (Chan) ]
                                  |
                                  v
             +-------------------------------------------+
             |            SSE Broker Goroutine           |
             |  - Menyimpan map client terdaftar         |
             |  - client: chan []byte                    |
             +-------------------------------------------+
                     /             |              \
                    v              v               v
             [ Client 1 ]    [ Client 2 ]    [ Client 3 ]
             (Browser A)     (Browser B)     (Browser C)
```

**Karakteristik Broker di Go:**
- Thread-safe dengan `sync.RWMutex`.
- Mampu mendeteksi client yang disconnect via `r.Context().Done()` dan otomatis membersihkan memory channel.
- Mengirimkan buffer ter-render dari komponen Templ.

### 5.2 Mengapa `Templ` Sangat Cocok dengan HTMX?
Templ mengkompilasi file `.templ` menjadi fungsi Go murni (`.go`). Keunggulannya:
1. **Type-Safe:** Tidak ada typo variabel di runtime. Jika Anda mengirim data `struct Metrics` dengan tipe salah, Go compiler akan menolaknya.
2. **Komposabilitas Komponen:** Komponen header, metric gauge, dan log table dipisah rapi seperti komponen React.
3. **High Performance:** Templ me-render langsung ke `io.Writer` (`http.ResponseWriter`), menghasilkan latency mikrodetik tanpa overhead reflection template engine konvensional.

---

## 6. Desain Dashboard "PulseOps"

Dashboard yang akan kita bangun memiliki tema **Sleek Cyber/Dark-mode Telemetry**:

### Layout Wireframe
```
+-----------------------------------------------------------------------------------+
|  [PulseOps Logo]   Live Node: srv-sg-01   [System Status OOB]   [Alert Badge OOB] |
+-----------------------------------------------------------------------------------+
| METRICS GRID (Live via SSE & OOB)                                                 |
| +--------------------+ +--------------------+ +--------------------+              |
| | CPU Usage          | | Memory Usage       | | Network I/O        |              |
| | [ 78.4% ] [Gauge]  | | [ 6.8 GB / 16GB ]  | | [ 142 MB/s ]       |              |
| +--------------------+ +--------------------+ +--------------------+              |
+-----------------------------------------------------------------------------------+
| LIVE EVENT & LOG STREAM                                                           |
| Filter: [All] [Info] [Warn] [Error]                  Status: Connected (SSE)      |
| +-------------------------------------------------------------------------------+ |
| | TIMESTAMP | LEVEL    | SERVICE   | MESSAGE                                    | |
| |-----------+----------+-----------+--------------------------------------------| |
| | 00:45:12  | INFO     | auth-api  | Token refreshed for usr_8812               | |
| | 00:45:14  | WARN     | db-pool   | Connection latency > 120ms                 | |
| | 00:45:16  | CRIT     | ingress   | Rate-limit exceeded on /checkout           | |
| +-------------------------------------------------------------------------------+ |
+-----------------------------------------------------------------------------------+
| [TOAST CONTAINER OOB (Floating bottom-right)]                                     |
|  - ⚠️ Rate-limit spike detected!                                                  |
+-----------------------------------------------------------------------------------+
```

---

## 7. Rencana Struktur Direktori Proyek

```
d:/Development/HTMX/
├── cmd/
│   └── server/
│       └── main.go           # Entry point aplikasi HTTP & SSE broker
├── internal/
│   ├── broker/
│   │   └── broker.go         # SSE Hub (Goroutines, channels, client registration)
│   ├── telemetry/
│   │   ├── collector.go      # Generator/collector data metrik & log
│   │   └── model.go          # Data structs (Metrics, EventLog, NodeState)
│   └── views/
│       ├── layout.templ      # Shell HTML dasar (HTML5, HTMX, Tailwind, SSE ext)
│       ├── dashboard.templ   # Halaman utama dashboard
│       └── components/       # Komponen partikel
│           ├── metrics.templ # Kartu CPU, RAM, Network (dengan target swap OOB)
│           ├── logs.templ    # Baris log stream
│           └── toast.templ   # Notifikasi pop-up dinamis
├── public/
│   ├── css/
│   │   └── style.css         # Output Tailwind CSS
│   └── js/
│       └── htmx-sse.js       # HTMX SSE Extension
├── docs/
│   └── SYSTEM_DESIGN_AND_EXPLORATION.md  # Dokumen arsitektur ini
├── Makefile                  # Perintah build, templ generate, air run
└── go.mod
```

---

## 8. Roadmap Pelaksanaan

1. **Fase 0: Environment Alignment**
   - Verifikasi Go di terminal & pasang tool pendukung (`templ`, `air` untuk hot-reload jika diinginkan).
2. **Fase 1: Go Module & Basic SSE Broker**
   - Inisialisasi `go.mod`.
   - Bangun SSE Broker murni di Go untuk menguji stream `text/event-stream` ke browser.
3. **Fase 2: Templ Component Hierarchy**
   - Susun template `layout.templ` dan komponen `metrics.templ`, `logs.templ`.
4. **Fase 3: Integrasi HTMX SSE & OOB Swap**
   - Pasang atribut `hx-ext="sse"`, `sse-connect`, dan buat handler server yang memancarkan multi-swap OOB.
5. **Fase 4: Visual Polish dengan Tailwind CSS**
   - Buat tampilan sleek dark-mode, animasi pulse, status gauges, dan toast slide-in effect.
