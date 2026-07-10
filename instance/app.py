import os
import asyncio
import requests
import websockets

# Lire la valeur de la variable d'environnement "HOST_KEY"
host_key = os.getenv("HOST_KEY")
print(host_key)

clients = set()
server = None

async def handler(websocket, path):
    global server
    clients.add(websocket)
    try:
        async for message in websocket:
            print("message reçu: " + message)
            if message == host_key:
                if server is None:
                    server = websocket
                    print("found")
                    await websocket.send("Vous êtes maintenant le gestionnaire de la partie.")
                else:
                    await websocket.send("Il y a déjà un gestionnaire de la partie.")
            else:
                if len(clients) > 1:
                    try:
                        await asyncio.wait([client.send(message) for client in clients if client != websocket])
                    finally:
                        pass
    except websockets.exceptions.ConnectionClosedError as e:
        print(f"Connexion fermée avec l'erreur : {e}")
    except Exception as e:
        print(f"Erreur inattendue : {e}")
    finally:
        clients.remove(websocket)
        if websocket == server:
            server = None
        if len(clients) == 0:
            stop()

async def main():
    async with websockets.serve(handler, "0.0.0.0", 8000):
        await asyncio.Future()  # run forever


def stop():
    instance_name = os.environ["INSTANCE_NAME"]
    backend_url = os.environ["BACKEND_URL"]
    password = os.environ["PASSWORD"]
    requests.post(f"https://{backend_url}/delete-room", data={"instance": instance_name, "password": password}, verify=False)
    return "deleted"

if __name__ == "__main__":
    asyncio.run(main())
