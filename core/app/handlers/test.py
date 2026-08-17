"""A basic (single function) API written using hug"""
import hug


@hug.get('/test')
def test(string, number:hug.types.number=1):
    return "string: {string}, number {number}!".format(**locals())