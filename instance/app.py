import os
import asyncio
import json
import requests
import websockets

# Lire la valeur de la variable d'environnement "HOST_KEY"
host_key = os.getenv("HOST_KEY")
print(host_key)

clients = set()
nicknames = {}
server = None

async def broadcast(message, exclude=None):
    if clients:
        await asyncio.gather(
            *[
                client.send(message)
                for client in clients
                if client != exclude
            ],
            return_exceptions=True
        )

async def handler(websocket):
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
                continue

            payload = None
            try:
                payload = json.loads(message)
            except json.JSONDecodeError:
                pass

            if isinstance(payload, dict):
                msg_type = payload.get("type")

                if msg_type == "join":
                    nickname = str(payload.get("nickname", "")).strip() or "Anonyme"
                    nicknames[websocket] = nickname
                    await broadcast(f"{nickname} a rejoint la room.", exclude=websocket)
                    await websocket.send(f"Bienvenue {nickname} dans la room.")
                    continue

                if msg_type == "message":
                    text = str(payload.get("text", "")).strip()
                    if not text:
                        continue
                    nickname = nicknames.get(websocket, "Anonyme")
                    await broadcast(f"{nickname}: {text}")
                    continue

            await broadcast(message)
    except websockets.exceptions.ConnectionClosedError as e:
        print(f"Connexion fermée avec l'erreur : {e}")
    except Exception as e:
        print(f"Erreur inattendue : {e}")
    finally:
        nickname = nicknames.pop(websocket, None)
        clients.remove(websocket)
        if websocket == server:
            server = None
        if nickname:
            print(f"{nickname} a quitté la room.")
            await broadcast(f"{nickname} a quitté la room.", exclude=websocket)
        print(f"Clients restants : {len(clients)}")
        if len(clients) == 0:
            stop()

async def main():
    async with websockets.serve(handler, "0.0.0.0", 8000):
        await asyncio.Future()  # run forever


def stop():
    print("Arrêt du serveur WebSocket car il n'y a plus de clients connectés.")
    instance_name = os.environ["INSTANCE_NAME"]
    backend_url = os.environ["BACKEND_URL"]
    password = os.environ["PASSWORD"]
    requests.post(f"https://{backend_url}/delete-room", data={"instance": instance_name, "password": password}, verify=False)
    return "deleted"

if __name__ == "__main__":
    asyncio.run(main())
