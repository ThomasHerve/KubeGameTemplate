import hug, os, random, string
from kubernetes import client, config
import qrcode
from PIL import Image
import redis
import time
import uuid


class RedisLock:
    def __init__(self, client, lock_key, lock_timeout=10):
        self.client = client
        self.lock_key = lock_key
        self.lock_timeout = lock_timeout
        self.lock_value = str(uuid.uuid4())

    def acquire_lock(self):
        while True:
            if self.client.set(self.lock_key, self.lock_value, nx=True, ex=self.lock_timeout):
                return True
            time.sleep(0.1)

    def release_lock(self):
        if self.client.get(self.lock_key) == self.lock_value:
            self.client.delete(self.lock_key)

# Configuration de la connexion Redis
redis_client = redis.StrictRedis(host=os.environ["REDIS_URL"], port=int(os.environ["REDIS_PORT"]), db=0)

lock = RedisLock(redis_client, 'lock')

@hug.post('/create-room')
def create_room():

    image = os.environ["INSTANCE_IMAGE"]
    namespace = os.environ["KUBERNETES_NAMESPACE"]
    ingress = os.environ["KUBERNETES_INGRESS_NAME"]
    port = int(os.environ["KUBERNETES_PORT"])
    extension = os.environ["EXTENSION"]
    external_port = int(os.environ["EXTERNAL_PORT"])
    internal_port = int(os.environ["INTERNAL_PORT"])

    # Random pod id
    pod_id = ''.join(random.choice(string.ascii_lowercase) for i in range(10)) 

    config.load_kube_config(config_file='./kubernetes-config')
    v1 = client.CoreV1Api()
	
    # Random key for host
    host_key = ''.join(random.choice(string.ascii_lowercase) for i in range(10)) 

    # Create a pod
    containers = []
    container1 = client.V1Container(name='instance', image=image, env=[client.V1EnvVar(name="INSTANCE_NAME", value=pod_id), client.V1EnvVar(name="BACKEND_URL", value=os.environ["BACKEND_URL"]), client.V1EnvVar(name="PASSWORD", value=os.environ["PASSWORD"]), client.V1EnvVar(name="HOST_KEY", value=host_key)])
    containers.append(container1)

    pod_spec = client.V1PodSpec(containers=containers)
    pod_metadata = client.V1ObjectMeta(name='instance-' + pod_id, namespace=namespace, labels={
        "pod_id": pod_id
    })

    pod_body = client.V1Pod(api_version='v1', kind='Pod', metadata=pod_metadata, spec=pod_spec)
        
    v1.create_namespaced_pod(namespace=namespace , body=pod_body)

    # Create a service
    service_port_list = [client.V1ServicePort(port=external_port, target_port=internal_port, name='http')]
    service_spec = client.V1ServiceSpec(ports=service_port_list, selector={
        "pod_id": pod_id
    })
    service_metadata = client.V1ObjectMeta(name='instance-' + pod_id, namespace=namespace)
    service = client.V1Service(metadata=service_metadata, spec=service_spec)
    
    v1.create_namespaced_service(namespace=namespace , body=service)

    # Update the ingress class
    # Needs a general lock on the ressource
    if lock.acquire_lock():
        try:
            networking = client.NetworkingV1Api()

            current_ingress = networking.read_namespaced_ingress(name=ingress, namespace=namespace)
            current_ingress_paths = current_ingress.spec.rules[0].http.paths
            current_ingress_paths.append(client.V1HTTPIngressPath(path=f"/{pod_id}{extension}", path_type="Prefix", backend=client.V1IngressBackend(service=client.V1IngressServiceBackend(name=f"instance-{pod_id}", port=client.V1ServiceBackendPort(number=port)))))
            current_ingress.spec.rules[0].http.paths = current_ingress_paths

            networking.patch_namespaced_ingress(ingress, namespace, current_ingress)
        finally:
            lock.release_lock() 

    # QR code
    url = value=os.environ["BACKEND_URL"] + "/" + pod_id
    qr = qrcode.QRCode(
        version=1,
        error_correction=qrcode.constants.ERROR_CORRECT_L,
        box_size=10,
        border=4,
    )
    qr.add_data(url)
    qr.make(fit=True)

    # Create a PIL image PIL from the QR code
    img = qr.make_image(fill_color="black", back_color="white")

    # convert the PIL image into an array
    qr_array = []
    width, height = img.size
    pixels = img.load()
    for y in range(height):
        row = []
        for x in range(width):
            # 1: black, 0: white
            row.append(1 if pixels[x, y] == 0 else 0)
        qr_array.append(row)

    return {"instance": pod_id, "qr_code": qr_array, "host_key": host_key}

@hug.post('/delete-room')
def delete_room(body):
    ingress = os.environ["KUBERNETES_INGRESS_NAME"]
    namespace = os.environ["KUBERNETES_NAMESPACE"]
    
    config.load_kube_config(config_file='./kubernetes-config')
    v1 = client.CoreV1Api()

    pods_list = v1.list_namespaced_pod(namespace=namespace)
    pods = [item.metadata.name for item in pods_list.items]
    if not f"instance-{body['instance']}" in pods:
        return "Instance " + body["instance"] + " does not exist"

    if "password" not in body or body["password"] != os.environ["PASSWORD"]:
        return "Wrong password"

    v1.delete_namespaced_pod(namespace=namespace, name='instance-'+body["instance"])
    v1.delete_namespaced_service(namespace=namespace, name='instance-'+body["instance"])

    # Remove ingress entry
    if lock.acquire_lock():
        try:
            networking = client.NetworkingV1Api()
            current_ingress = networking.read_namespaced_ingress(name=ingress, namespace=namespace)
            current_ingress_paths = current_ingress.spec.rules[0].http.paths
            current_ingress_paths = list(filter(lambda x: x.backend.service.name != f"instance-{body['instance']}", current_ingress_paths))
            current_ingress.spec.rules[0].http.paths = current_ingress_paths

            networking.patch_namespaced_ingress(ingress, namespace, current_ingress)
        finally:
            lock.release_lock() 

    return "Ok"
