# ⚡ PulseOps Enterprise: Real-Time Telemetry & Chaos Platform (GoTH Stack)

A cutting-edge, real-time infrastructure telemetry & incident management platform showcasing the full power of **HTMX 2.0 Server-Sent Events (SSE)**, **Out-of-Band (OOB) Swaps**, and **Server-Rendered Pure SVG Graphics** without React/Vue or complex client-side state managers.

---

## 🚀 Tech Stack ("GoTH")

- **Go 1.25+**: Ultra-fast backend, thread-safe SSE Fan-out Broker, Goroutine telemetry simulation.
- **[Templ](https://templ.guide)**: Compile-time type-safe HTML component engine.
- **[HTMX 2.0](https://htmx.org)**: Hypermedia engine with `hx-ext="sse"`, `hx-swap-oob="true"`, `hx-push-url="true"`, and active debounced search.
- **Tailwind CSS**: Modern cyber dark-mode interface with micro-interactions.
- **Zero Frontend Chart Libraries**: Real-time SVG Sparklines rendered 100% on the server!

---

## 🌟 Fitur Baru & Unggulan

### 1. 📈 Server-Rendered SVG Sparklines (Zero Client JS!)
- Kurva tren CPU di kartu telemetri dirender langsung oleh Go + Templ sebagai elemen `<svg viewBox="...">` dengan polyline & linear-gradient.
- Setiap tick SSE (1.5s), server mengalirkan potongan SVG baru yang bertransisi halus tanpa butuh Chart.js atau D3.js.

### 2. 🌐 Multi-Cluster Node Switcher (`hx-push-url="true"`)
- Switch antar cluster: `Singapore Core Hub`, `US-East Edge Shard`, dan `Frankfurt Storage Core`.
- Menggunakan `hx-post="/api/node/{id}/select"` yang memperbarui grid metrik secara atomik dan menyinkronkan URL browser (`?node=...`) tanpa full-page reload!

### 3. 🚨 Interactive Incident Center ("Click to Resolve")
- Insiden kritis dan peringatan otomatis masuk ke panel insiden via SSE.
- Operator dapat mengklik:
  - **`👁️ Acknowledge`**: Mengubah status insiden langsung di tempat.
  - **`✓ Resolve`**: Menghapus kartu insiden dari DOM, mengurangi counter badge di header via OOB, dan menyisipkan entri audit log ke tabel log via OOB swap!

### 4. 🧪 Chaos Engineering Suite
- Uji coba ketahanan sistem dengan simulasi kegagalan satu klik:
  - **`🔥 CPU Spike`**: Memaksa CPU ke 99.2% dan memicu critical incident.
  - **`💧 Memory Leak`**: Mengisi buffer RAM hingga 94%.
  - **`🔌 Network Flap`**: Menurunkan throughput jaringan ke 0.4 MB/s.
  - **`🛡️ Self-Heal All`**: Merestorasi seluruh node ke status nominal dan membersihkan semua insiden.

### 5. 🔍 Active Search & Severity Filter pada Log Stream
- Input pencarian log dengan debounce otomatis (`hx-trigger="keyup changed delay:300ms, search"`).
- Filter instan berdasarkan level: `[ALL]`, `[WARN]`, `[CRIT]`.

---

## 🛠️ Cara Menjalankan

Cukup jalankan script:
```powershell
.\run.ps1
```
Atau akses langsung di browser jika server sedang berjalan:  
👉 **[http://localhost:8080](http://localhost:8080)**
