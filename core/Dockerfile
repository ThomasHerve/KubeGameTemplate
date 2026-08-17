FROM python:3.9

EXPOSE 8000

COPY ./core/requirements.txt requirements.txt
RUN pip3 install gunicorn
RUN pip3 install -r requirements.txt
WORKDIR /src

RUN mkdir -p /src
COPY ./core/app/ /src

CMD gunicorn --reload --bind=0.0.0.0:8000 __init__:__hug_wsgi__
