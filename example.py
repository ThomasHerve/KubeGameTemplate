import requests
import tk

backend_url = "asyncunitycore.multiplayertournamentonline.fr"
value = requests.post(f"https://{backend_url}/create-room")

value = value.json()
instance = value["instance"]
qr_code = value["qr_code"]

requests.post(f"https://{backend_url}/{instance}/stop")

def draw_qr_code(canvas, qr_code, pixel_size=10):
    rows = len(qr_code)
    cols = len(qr_code[0])

    for row in range(rows):
        for col in range(cols):
            color = "black" if qr_code[row][col] == 1 else "white"
            x0 = col * pixel_size
            y0 = row * pixel_size
            x1 = x0 + pixel_size
            y1 = y0 + pixel_size
            canvas.create_rectangle(x0, y0, x1, y1, fill=color, outline="")


# Créer une fenêtre Tkinter
root = tk.Tk()
root.title("QR Code")

# Créer un canevas pour dessiner le QR code
canvas_width = len(qr_code[0]) * 10  # largeur du canevas
canvas_height = len(qr_code) * 10    # hauteur du canevas
canvas = tk.Canvas(root, width=canvas_width, height=canvas_height, bg="white")
canvas.pack()

# Dessiner le QR code sur le canevas
draw_qr_code(canvas, qr_code)

# Lancer la boucle principale de Tkinter
root.mainloop()
