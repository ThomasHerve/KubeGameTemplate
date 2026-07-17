# PLAN de migration vers API Gateway

## Contexte

Le projet utilise aujourd’hui un modèle basé sur un Ingress Kubernetes pour exposer le frontend, le service `core` et des instances dynamiques créées par le module `core/app/handlers/core.py`.

Le module `core` gère la création et la destruction de pods d’instance et met à jour l’Ingress en ajoutant ou retirant des chemins dynamiques pour chaque session.

## Objectif

Migrer l’architecture pour utiliser une API Gateway au lieu de l’Ingress classique, afin de :

- centraliser la gestion des routes HTTP/WebSocket,
- simplifier le routage dynamique des instances,
- mieux contrôler l’authentification, les certificats et les réécritures de chemin,
- réduire la dépendance à la modification directe d’un objet `Ingress`.

## État actuel important

- `helm/templates/ingress.yaml` génère un Ingress basé sur `Values.ingress.hosts`.
- `helm/templates/deployment.yaml` injecte `KUBERNETES_INGRESS_NAME` et `BACKEND_URL` depuis les valeurs `ingress.hosts[0]`.
- `core/app/handlers/core.py` utilise l’API Kubernetes pour patcher l’Ingress et ajouter des chemins dynamiques vers les pods créés.
- `frontend/public/index.html` et `frontend/public/config.template.js` utilisent `APP_CONFIG.BACKEND_URL` pour accéder au backend.

## Grandes étapes de migration

### 1. Choisir l’API Gateway cible

- Identifier la passerelle adaptée au cluster : Traefik, Kong, Ambassador, Gloo, Istio Gateway, AWS API Gateway, etc.
- Vérifier si elle expose des CRD/ressources spécifiques pour déclarer des routes et services.

### 2. Remplacer l’Ingress par des ressources Gateway

- Conserver `helm/templates/ingress.yaml` comme option de rollback pendant la migration, mais désactiver `Values.ingress.enabled` par défaut.
- Ajouter un template `helm/templates/httproute.yaml` inspiré du chart `label-ui` pour générer des `HTTPRoute` Gateway API.
- Définir un bloc `httpRoute:` dans `helm/values.yaml` avec :
  - `enabled`,
  - `annotations`,
  - `parentRefs`,
  - `hostnames`,
  - `rules`.
- Si la gateway cible ne supporte pas `HTTPRoute`, remplacer le template par le CRD adapté (`IngressRoute`, `VirtualService`, etc.).

### 3. Adapter les services exposés

- Garder les Services Kubernetes (`core`, `frontend`, `instance-fixe`, `redis`) en `ClusterIP`.
- Ne plus exposer les services via `Ingress` dans le plan de production principal.
- L’API Gateway doit router uniquement vers les services internes.
- Si l’API Gateway expose un contrôleur `Gateway` commun, utiliser un objet `Gateway` / `GatewayClass` externe au chart et ne pas exposer directement les services.

### 4. Adapter le chart Helm

- Ajouter une section `gateway:` ou `httpRoute:` dans `helm/values.yaml`.
- Garder `helm/templates/service.yaml` inchangé pour le routage interne.
- Ajouter `helm/templates/httproute.yaml` comme dans `label-ui`, avec `parentRefs`, `hostnames`, `matches`, `filters` et `backendRefs`.
- Mettre à jour `helm/templates/deployment.yaml` :
  - `BACKEND_URL` doit pointer vers l’hôte du gateway ou du domaine public,
  - `BACKEND_PROTOCOL` doit être `http` ou `https` selon la terminaison TLS du gateway.
- Supprimer `KUBERNETES_INGRESS_NAME` et les références à l’Ingress si la gestion dynamic-route est externalisée.
- Prévoir des valeurs de fallback : `ingress.enabled` `false` et `httpRoute.enabled` `true`.

### 5. Changer le code de gestion dynamique des instances

- Dans `core/app/handlers/core.py`, remplacer la logique de patch d’Ingress par une gestion des ressources Gateway :
  - création d’un objet `HTTPRoute` ou appel à l’API de configuration de la gateway pour chaque instance,
  - ou stockage d’une route dans Redis/etcd si la gateway a un contrôleur spécifique.
- Pour chaque instance :
  - créer le pod et le service Kubernetes standard,
  - créer la route Gateway pointant vers `instance-<id>` ou vers un service dédié,
  - supprimer la route au `delete-room`.
- Si la passerelle offre un endpoint REST ou un CRD, utiliser ce mécanisme plutôt que `NetworkingV1Api().patch_namespaced_ingress()`.
- Assurer que l’instance nouvellement créée renvoie bien l’URL publique de la gateway.

### 6. Gérer les URL et chemins dynamiques

- Choisir un modèle de route compatible avec l’API Gateway :
  - sous-domaine dynamique (`instance-<id>.example.com`), ou
  - préfixe de chemin (`/<id>` ou `/instance/<id>`).
- Dans la logique `core`, générer l’URL en utilisant le domaine/host configuré par le gateway.
- Mettre à jour la génération de QR code pour pointer vers l’URL Gateway publique.
- Vérifier que le parsing du chemin est compatible avec le `pathType` et les règles du gateway.

### 7. Vérifier la configuration du frontend

- `frontend/public/config.template.js` doit charger l’hôte public du gateway.
- `frontend/public/index.html` doit utiliser la base `BACKEND_URL` fournie par le gateway pour :
  - `POST /create-room`,
  - `WebSocket` vers la room.
- Si la gateway termine TLS, adapter le protocole WebSocket à `wss://`.
- S’assurer que l’URL du backend dynamique est accessible depuis le navigateur via le gateway.

### 8. Tester la migration

- Déployer une version de test du chart avec `httpRoute.enabled=true` et `ingress.enabled=false`.
- Vérifier :
  - déploiement du frontend et du backend via la gateway,
  - création de room et création d’instances,
  - routage dynamique des instances via `HTTPRoute` ou la ressource gateway choisie,
  - suppression de routes au `delete-room`.
- Contrôler que les services internes restent en `ClusterIP` et ne sont pas exposés directement.
- Comparer le comportement avec la configuration Ingress de rollback.

### 9. Nettoyer l’ancienne logique Ingress

- Garder le template `helm/templates/ingress.yaml` uniquement pour rollback ou migration progressive.
- Retirer les valeurs `ingress.*` inutilisées dans `helm/values.yaml`.
- Mettre à jour `core/README.md`, `README.md` et `example.py` pour documenter la nouvelle architecture Gateway.
- Documenter l’usage de `httpRoute.enabled` et les prérequis de la gateway.

## Points techniques à documenter

- `core/app/handlers/core.py`
  - arrêt de la modification directe d’Ingress,
  - création / suppression d’un route object Gateway.
- `helm/values.yaml`
  - ajout de `gateway:` pour l’hôte et le contrôleur,
  - adaptation des services exposés.
- `helm/templates/deployment.yaml`
  - injection des nouveaux endpoints de gateway dans `BACKEND_URL`.
- `frontend/public/config.template.js`
  - configuration de l’URL du gateway.
- `helm/templates/service.yaml`
  - confirm services `ClusterIP` uniquement, si le gateway est externe.

## Risques et pièges

- Les API Gateway ont souvent des CRD différentes : la migration doit être spécifique au moteur choisi.
- Les règles de réécriture peuvent changer : vérifier le comportement WebSocket et HTTP.
- La gestion TLS peut être centralisée dans la gateway, ce qui nécessite d’ajuster `BACKEND_PROTOCOL`.
- Si le gateway ne supporte pas la modification dynamique via API, il faudra prévoir une couche de synchronisation ou un opérateur.

## Livraison proposée

1. Prototyper sur un cluster avec une API Gateway choisie.
2. Adapter le chart Helm pour exposer les services via la gateway.
3. Mettre à jour `core/app/handlers/core.py` pour la création dynamique de routes.
4. Mettre à jour le frontend pour pointer vers le gateway.
5. Supprimer l’ancien Ingress et valider le comportement.
6. Documenter la nouvelle architecture dans `README.md` et dans ce plan.
