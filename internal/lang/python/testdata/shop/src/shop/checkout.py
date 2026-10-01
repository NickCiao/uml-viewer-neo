"""Checkout: three paths in run — the function, the loop, the condition."""
import json
from typing import List

try:  # a third-party import that the fixture's tests can live without
    import requests
except ImportError:
    requests = None

from shop import cart
from shop.cart.basket import Basket
from .pricing import total


def run(items: List[str]) -> int:
    basket = Basket()
    for item in items:
        if item:
            basket.add(item)
    return total(basket)


def never_called():
    return json.dumps(str(requests))
