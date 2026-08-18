# TODO: Séparation des routes Go et activation conditionnelle

## Objectif
Refactoriser l'architecture des routes pour séparer les responsibilities et permettre une activation/désactivation conditionnelle des endpoints.

## Contexte actuel
- Un seul endpoint HTTP qui gère create-room et delete-room
- L'API exposée via le service `instance`
- Les instances communiquent via l'API

## Nouveau design
```
Frontend/Web                    Instance Service (Python)
        │                               │
        ├──► POST /create-room ────────┤
        │                               ├──► Backend Go (route create)
        │                               │
        └──► POST /delete-room ────────┤
                                        │
Backend/Instance (Go) ──────────────────┘
  (communication interne directe via URL)
```

## Étapes d'implémentation

### Phase 1: Ajouter configuration d'activation des routes

**Fichier:** `operator/api/v1alpha1/gameinstancesmanager_types.go`

```go
type InstanceSpec struct {
    // ... champs existants ...
    
    // Nouvelle section: Configuration des routes
    Routes *RoutesConfig `json:"routes,omitempty"`
}

type RoutesConfig struct {
    // Route pour créer des instances
    // +optional
    // +kubebuilder:default=true
    CreateEnabled bool `json:"createEnabled,omitempty"`
    
    // Route pour supprimer des instances
    // +optional
    // +kubebuilder:default=false
    DeleteEnabled bool `json:"deleteEnabled,omitempty"`
}
```

### Phase 2: Séparer les routes dans l'API Python

**Fichier:** `operator/internal/api/server.go`

Actions:
  
- Changer la variable passer par le crd à l'instance pour utiliser `http://<SERVICE NAME>.<SERVICE NAMESPACE NAME>.svc.cluster.local:8080` au lieu du hostname (éviter de passer par la route externe)


### Phase 3: Créer les deux routes Go dans l'opérateur

**Fichier:** `operator/internal/api/server.go`

Actions:
- Créer deux handlers distincts au lieu d'un:
  ```go
  func handleCreateRoom(w http.ResponseWriter, r *http.Request)  // Existant, isoler la logique
  func handleDeleteRoom(w http.ResponseWriter, r *http.Request)  // Existant, isoler la logique
  ```

- Ajouter une fonction pour récupérer l'état des routes depuis le CRD:
  ```go
  func isRouteEnabled(routeName string, gim *GameInstancesManager) bool
  ```

- Ajouter une middleware pour vérifier si une route est activée:
  ```go
  func routeEnabled(routeName string) http.Handler
  ```

### Phase 4: Passer l'URL du service Go aux instances

**Fichier:** `operator/internal/controller/gameinstancesmanager_controller.go`

Actions:
- Lors de la création du Deployment d'une instance, ajouter une variable d'environnement:
  ```go
  {
    Name: "BACKEND_GO_URL",
    Value: fmt.Sprintf("http://%s-api:8080", gim.Name),
    // ou depuis une configMap
  }
  ```

- Cette URL pointe vers un nouveau service Go qui expose create-room et delete-room

### Phase 5: Créer un service Kubernetes pour les routes Go

**Fichier:** `operator/internal/controller/gameinstancesmanager_controller.go` ou nouveaux helpers

Actions:
- Ajouter une fonction `reconcileBackendService()`:
  ```go
  func (r *GameInstancesManagerReconciler) reconcileBackendService(ctx context.Context, gim *GameInstancesManager) error
  ```

- Crée un Service qui expose le port 8080 du pod de l'opérateur
- Sélecteur: labels du contrôleur
- Nom: `<gim.name>-api`

### Phase 6: Configuration dans le CRD

**Exemple d'utilisation:**

```yaml
apiVersion: apps.thomas-herve.fr/v1alpha1
kind: GameInstancesManager
metadata:
  name: my-game
spec:
  instance:
    repository: thomasherve/kube-game-instance
    tag: "latest"
    internalPort: 8000
    # Configuration des routes
    routes:
      createEnabled: true    # Frontend peut créer des instances
      deleteEnabled: false   # Frontend NE peut PAS supprimer
                             # (l'instance le fait directement via BACKEND_GO_URL)
  # ... reste du spec ...
```

### Phase 7: Mise à jour du Helm chart et values

**Fichier:** `helm/values.yaml`

Actions:
- Ajouter des valeurs par défaut pour les routes:
  ```yaml
  instance:
    routes:
      createEnabled: true
      deleteEnabled: false
  ```

- Mettre à jour les templates pour passer ces valeurs au CRD

### Phase 8: Transparence pour l'applicatif actuel

**Assurer la backward compatibility:**

- Les deux endpoints `/create-room` et `/delete-room` doivent toujours exister par défaut
- Par défaut: `createEnabled=true`, `deleteEnabled=false`
- Les vieilles instances qui utilisent l'API Python reçoivent `BACKEND_GO_URL` mais ne sont pas obligées de l'utiliser
- Les nouvelles instances peuvent communiquer directement si `deleteEnabled=false`

## Fichiers à modifier

```
operator/
  ├── api/v1alpha1/
  │   ├── gameinstancesmanager_types.go        (+ champs Routes/RoutesConfig)
  │   └── zz_generated.deepcopy.go             (auto-généré)
  ├── internal/
  │   ├── api/server.go                        (routes séparées, middleware activation)
  │   └── controller/
  │       └── gameinstancesmanager_controller.go (reconcileBackendService, BACKEND_GO_URL)
  ├── config/samples/
  │   └── apps_v1alpha1_gameinstancesmanager.yaml (exemple)
  └── Makefile                                  (make generate)

instance/
  ├── app.py                                   (env vars ROUTES_*, BACKEND_GO_URL)
  ├── Dockerfile
  └── requirements.txt

helm/
  ├── values.yaml                              (routes.createEnabled, deleteEnabled)
  ├── Chart.yaml
  └── templates/
      └── crds/
          └── apps_v1alpha1_gameinstancesmanager.yaml (mise à jour)
```

## Avantages de ce design

✅ **Flexibilité**: Chaque déploiement peut choisir comment activer les routes
✅ **Transparence**: Les applications existantes continuent de fonctionner
✅ **Sécurité**: L'instance peut communiquer directement sans passer par l'API publique
✅ **Scalabilité**: Les deux endpoints peuvent être sur des services différents
✅ **Maintenabilité**: Séparation claire des concerns

## Notes importantes

- Après les modifications du CRD, exécuter: `make generate` dans `operator/`
- Mettre à jour les RBAC si nécessaire (création/gestion du nouveau service)
- Tester la backward compatibility avec des images anciennes et nouvelles
- Documenter le nouveau comportement dans le README principal
