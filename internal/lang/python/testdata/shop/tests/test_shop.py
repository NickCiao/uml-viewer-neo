from shop.cart.basket import Basket
from shop.checkout import run


def test_run():
    run(["a", ""])


def test_full():
    b = Basket()
    b.add("a")
    b.add("b")
    assert not b.full()
    b.add("c")
    assert b.full()
