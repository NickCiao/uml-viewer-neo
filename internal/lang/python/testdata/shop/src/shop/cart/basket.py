class Basket:
    def __init__(self):
        self.items = []

    def add(self, item):
        self.items.append(item.strip())

    def full(self):
        return len(self.items) >= 3

    def __len__(self):
        return len(self.items)
