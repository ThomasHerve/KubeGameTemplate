L'objectif du module "core", est d'offrir un point d'entré unique au jeu dans le but d'instancier une nouvelle partie sur notre cluster, via la création d'un pod unique et d'un ingress dédié.
Unity pourra donc récupérer cette url et la convertir en QR Code pour permettre aux joueurs mobile de se connecter en tant que client à l'instance de jeu.

Exemple Ingress:
paths:
  - backend:
      service:
        name: instance-odiimbbrjo
        port:
          number: 8000
    path: /odiimbbrjo(/|$)(.*)
    pathType: Prefix
  - backend:
      service:
        name: core
        port:
          number: 8000
    path: /()(.*)
    pathType: Prefix