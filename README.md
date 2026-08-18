# KubeGameTemplate

Un système complet pour déployer et gérer des instances de jeu multijoueur asymétrique sur Kubernetes.

## Table des matières

- [Vue d'ensemble](#vue-densemble)
- [Architecture](#architecture)
- [Prérequis](#prérequis)
- [Installation](#installation)
- [Configuration](#configuration)
- [Utilisation](#utilisation)
- [Développement](#développement)
- [Structure du projet](#structure-du-projet)

## Vue d'ensemble

KubeGameTemplate fournit une infrastructure complète pour:

1. **Déployer un jeu principal** (backend + frontend web)
2. **Créer/détruire dynamiquement des instances de jeu** via une API REST
3. **Gérer les ressources Kubernetes** automatiquement avec un opérateur custom
4. **Limiter le nombre d'instances simultanées** pour contrôler les ressources

Le système permet à un jeu principal de tourner en permanence, tandis que des instances peuvent être créées à la demande par les utilisateurs via l'API.

### Cas d'usage

- Game jams: permettre au public d'interagir avec un jeu en direct
- Jeux collaboratifs: backend central avec plusieurs joueurs connectés
- Démonstrations interactives avec création dynamique de sessions

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────┐         ┌─────────────────────┐   │
│  │   Frontend Service   │         │   Instance Manager  │   │
│  │  (Web UI, Nginx)     │         │   API REST (:8080)  │   │
│  │  :80 / :443          │         │                     │   │
│  └──────────────────────┘         └─────────────────────┘   │
│           │                                 │               │
│           │                          Creates/Deletes        │
│           │                                 │               │
│  ┌────────┴─────────────────────────────────┴──────────┐    │
│  │                                                     │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │    │
│  │  │  Instance   │  │  Instance   │  │  Instance   │  │    │
│  │  │  Deployment │  │  Deployment │  │  Deployment │  │    │
│  │  │    (Pod)    │  │    (Pod)    │  │    (Pod)    │  │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │    │
│  │                                                     │    │
│  │  Créées/gérées par GameInstancesManager CRD         │    │
│  │  via l'opérateur                                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐   │
│  │  Opérateur Kubernetes (GameInstancesManager)         │   │
│  │  - Reconciliation des CRD                            │   │
│  │  - Création/suppression automatique des pods         │   │
│  │  - Gestion des services et routes                    │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Composants

#### 1. Opérateur (`operator/`)
- Contrôleur Kubernetes écrit en Go (kubebuilder)
- Réconcilie les ressources `GameInstancesManager`
- Crée/modifie/supprime automatiquement les Deployments, Services et HTTPRoutes
- Gère le frontend si necessaire

#### 2. API Instance (`instance/`)
-  Projet d'exemple d'instance de jeu
-  Dans ce cas ci sert de serveur de chat textuel
  
#### 3. Frontend (`frontend/`)
- Projet d'exemple pour un frontend
- Est l'interface utilisateur pour créer une room/rejoindre une room
- Dans ce cas ci le projet d'exemple est une petite application de chat textuel

#### 4. Helm Chart (`helm/`)
- Templates pour déployer l'infrastructure
- Configurable via `values.yaml`

## Prérequis

### Environnement local (développement)
- **Go** 1.24+
- **Python** 3.9+
- **Docker** 17.03+
- **Make**

### Cluster Kubernetes (production)
- **Kubernetes** 1.23+
- **kubectl** 1.23+
- Accès admin au cluster
- (Optionnel) **Gateway API** pour les HTTPRoute

## Installation

Vous pouvez aussi passer par argocd (ou autre système gitops). Cela permet d'éviter d'avoir à cloner le projet en local.
N'hésitez pas à créer vos values pour customiser le projet, comme le namespace par exemple.

### 1. Installer l'operator via Helm

```bash
cd helm

# Créer le namespace
kubectl create namespace operator-dev

# Installer le chart
helm install kube-game-operator ./operator/helm -n operator-dev
```

### 2. Installer le projet d'exemple

```bash
cd helm

# Créer le namespace
kubectl create namespace operator

# Installer le chart
helm install kube-game ./helm -n operator
```

## Configuration

Le projet sous `/helm` contient déjà un exemple (attention au hostname, mettez le votre)

### Créer une instance GameInstancesManager

Créez un fichier `game-instance.yaml`:

```yaml
apiVersion: apps.thomas-herve.fr/v1alpha1
kind: GameInstancesManager
metadata:
  name: my-game
  namespace: operator
spec:
  hostname: game.example.com
  
  # Configuration du backend (instances de jeu)
  instance:
    repository: thomasherve/kube-game-instance
    tag: "latest"
    pullPolicy: Always
    internalPort: 8000
  
  # Configuration du frontend (interface web)
  frontend:
    enabled: true
    repository: thomasherve/kube-game-frontend
    tag: "latest"
    port: 8080              # Port interne du conteneur
    externalport: 80        # Port exposé au public
    pullPolicy: Always
    backendProtocol: https
    replicas: 1
  
  # OPTIONNEL: Limiter le nombre d'instances simultanées
  # Si non défini, pas de limite
  maxInstances: 20
```

Appliquer:

```bash
kubectl apply -f game-instance.yaml
```

Vérifier le déploiement:

```bash
kubectl get gameinstancesmanagers -n operator
kubectl get deployments -n operator
kubectl get pods -n operator
```

## Utilisation

### Créer une instance de jeu

```bash
curl -X POST http://game.example.com/create-room
```

Réponse:

```json
{
  "instance": "abc12",
  "host_key": "xk9mq8p2r4t1v0w5"
}
```

### Supprimer une instance

```bash
curl -X POST http://game.example.com/delete-room \
  -d "instance=abc12&password=<password>"
```

Pour information seul votre instance obtiendra le mot de passe pour se supprimer.

TODO: Permettre de choisir d'exposer les routes de création et de destruction d'instance.

### Via le formulaire web

Le frontend expose une interface pour créer/supprimer des instances.

## Développement

### Structure des répertoires

```
.
├── operator/              # Opérateur Kubernetes (Go)
│   ├── api/v1alpha1/     # CRD definitions
│   ├── internal/         # Logique du contrôleur
│   ├── config/           # Manifests Kubernetes
│   ├── Makefile          # Build et tests
│   └── ...
├── instance/             # API Instance (Python)
│   ├── app.py            # Application Flask
│   ├── requirements.txt
│   ├── Dockerfile
│   └── ...
├── frontend/             # Interface web (Nginx)
│   ├── nginx.conf
│   ├── public/           # Fichiers statiques
│   └── Dockerfile
├── helm/                 # Chart Helm
│   ├── values.yaml
│   ├── Chart.yaml
│   └── templates/
└── README.md
```

### Modifier le CRD

Éditer `operator/api/v1alpha1/gameinstancesmanager_types.go`, puis:

```bash
cd operator
make generate
make manifests
```

### Lancer l'opérateur localement

```bash
cd operator
make run
```

### Tester

```bash
cd operator
make test
```

### Builder les images Docker

```bash
# Opérateur
cd operator
make docker-build IMG=kube-game-operator:dev

# Instance
cd instance
docker build -t kube-game-instance:dev .

# Frontend
cd frontend
docker build -t kube-game-frontend:dev .
```

## Points clés

- **Pas de limite par défaut**: Si `maxInstances` n'est pas défini, créez autant d'instances que vous le souhaitez
- **Gestion automatique**: L'opérateur crée/supprime les ressources Kubernetes automatiquement
- **API stateless**: Les instances sont sans état, idéal pour les jeux éphémères
- **Scalable**: Peut gérer des centaines d'instances sur un grand cluster

## Dépannage

### Les pods ne se créent pas

```bash
# Vérifier les logs de l'opérateur
kubectl logs -n operator deployment/deployment-dev-controller-manager

# Vérifier les CRD
kubectl get crd | grep gameinstancesmanagers

# Vérifier l'état du GameInstancesManager
kubectl describe gameinstancesmanager my-game -n operator
```

### Erreur "maximum instances reached"

Augmentez `maxInstances` dans votre CRD:

```bash
kubectl patch gameinstancesmanager my-game -n operator -p '{"spec":{"maxInstances":50}}'
```
