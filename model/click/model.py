import os
from pathlib import Path
from typing import Dict, List, Tuple

import click
import torch
from tqdm import tqdm


class Dataset:

    def __init__(self):
        self.indices = []
        self.values = []
        self.targets = []
        self.num_fields = 0
        self.num_features = 0

    def __len__(self):
        assert len(self.indices) == len(self.targets)
        return len(self.indices)

    def aligned(self) -> Tuple[List[List[int]], List[List[float]]]:
        aligned_indices = []
        aligned_values = []
        for i in range(len(self)):
            aligned_indices_row = self.indices[i]
            aligned_values_row = self.values[i]
            if len(aligned_indices_row) < self.num_fields:
                aligned_indices_row += [0] * (
                        self.num_fields - len(aligned_indices_row)
                )
                aligned_values_row += [0] * (self.num_fields - len(aligned_values_row))
            aligned_indices.append(aligned_indices_row)
            aligned_values.append(aligned_values_row)
        return aligned_indices, aligned_values


def load_libfm(path: str) -> Dataset:
    dataset = Dataset()
    with open(path, 'r') as f:
        for line in f.readlines():
            splits = line.strip().split(' ')
            indices = [int(v.split(':')[0]) for v in splits[1:]]
            values = [float(v.split(':')[1]) for v in splits[1:]]
            target = 1 if float(splits[0]) == 1 else 0
            dataset.indices.append(indices)
            dataset.values.append(values)
            dataset.targets.append(target)
            dataset.num_fields = max(dataset.num_fields, len(indices))
            dataset.num_features = max(dataset.num_features, max(indices) + 1)
    return dataset


def load_dataset(name: str) -> Tuple[Dataset, Dataset]:
    dataset_dir = os.path.join(Path.home(), '.gorse', 'dataset', name)
    return load_libfm(os.path.join(dataset_dir, "train.libfm")), load_libfm(os.path.join(dataset_dir, "test.libfm"))


def accuracy(positive_predictions: List[float], negative_predictions: List[float]) -> float:
    num_pos = len(positive_predictions)
    num_neg = len(negative_predictions)
    num_correct = 0
    for pos in positive_predictions:
        if pos > 0:
            num_correct += 1
    for neg in negative_predictions:
        if neg <= 0:
            num_correct += 1
    return num_correct / (num_pos + num_neg)


def auc(positive_predictions: List[float], negative_predictions: List[float]) -> float:
    sorted_positive_predictions = sorted(positive_predictions)
    sorted_negative_predictions = sorted(negative_predictions)
    sum = 0.0
    num_pos = 0
    for pos in sorted_positive_predictions:
        while (
                num_pos < len(sorted_negative_predictions)
                and sorted_negative_predictions[num_pos] < pos
        ):
            num_pos += 1
        sum += num_pos
    return sum / (len(positive_predictions) * len(negative_predictions))


class Evaluator:

    def __init__(self, test: Dataset, device: str = "cpu") -> None:
        align_indices, align_values = test.aligned()
        self.indices = torch.tensor(align_indices, dtype=torch.long).to(device)
        self.values = torch.tensor(align_values, dtype=torch.float).to(device)
        self.positive_samples = []
        self.negative_samples = []
        for i in range(len(test)):
            if test.targets[i] > 0:
                self.positive_samples.append(i)
            else:
                self.negative_samples.append(i)

    def evaluate(self, model) -> Dict[str, float]:
        positive_predictions = (
            model.predict(
                self.indices[self.positive_samples],
                self.values[self.positive_samples],
            )
            .cpu()
            .tolist()
        )
        negative_predictions = (
            model.predict(
                self.indices[self.negative_samples],
                self.values[self.negative_samples],
            )
            .cpu()
            .tolist()
        )
        return {
            "Accuracy": accuracy(positive_predictions, negative_predictions),
            "AUC": auc(positive_predictions, negative_predictions),
        }


class FactorizationMachine(torch.nn.Module):

    def __init__(
            self, num_fields: int, num_features: int, k: int = 8, init_stddev=0.01
    ) -> None:
        super().__init__()
        self.b = torch.nn.Parameter(torch.zeros(1))
        self.w = torch.nn.Embedding(num_features, 1)
        self.v = torch.nn.Embedding(num_features, k)
        torch.nn.init.normal_(self.w.weight, mean=0.0, std=init_stddev)
        torch.nn.init.normal_(self.v.weight, mean=0.0, std=init_stddev)

    def forward(self, indices, values):
        linear = self.b + torch.sum(self.w(indices) * values.unsqueeze(-1), dim=1)
        x = self.v(indices) * values.unsqueeze(-1)
        square_of_sum = torch.sum(x, dim=1) ** 2
        sum_of_square = torch.sum(x ** 2, dim=1)
        interaction = 0.5 * torch.sum(
            square_of_sum - sum_of_square, dim=1, keepdim=True
        )
        fm = linear + interaction
        return fm.squeeze()

    def predict(self, indices, values):
        return self.forward(indices, values)

    def fit(
            self,
            train: Dataset,
            test: Dataset,
            epochs: int = 20,
            lr: float = 0.01,
            reg: float = 0.0001,
            batch_size: int = 1024,
            device: str = "cpu",
            verbose: bool = True,
    ) -> dict[str, float]:
        self.to(device)
        aligned_indices, aligned_values = train.aligned()
        indices = torch.tensor(aligned_indices, dtype=torch.long).to(device)
        values = torch.tensor(aligned_values, dtype=torch.float).to(device)
        targets = torch.tensor(train.targets, dtype=torch.float).to(device)

        optimizer = torch.optim.Adam(self.parameters(), lr=lr, weight_decay=reg)
        criterion = torch.nn.BCEWithLogitsLoss()
        evaluator = Evaluator(test, device)

        score = evaluator.evaluate(self)

        for epoch in range(epochs):
            for i in tqdm(
                    range(0, len(indices), batch_size),
                    disable=not verbose,
                    desc=f"Epoch {epoch + 1}/{epochs}",
                    postfix=score,
            ):
                batch_indices = indices[i: i + batch_size]
                batch_values = values[i: i + batch_size]
                batch_targets = targets[i: i + batch_size]
                optimizer.zero_grad()
                output = self.forward(batch_indices, batch_values)
                loss = criterion(output, batch_targets)
                loss.backward()
                optimizer.step()
            score = evaluator.evaluate(self)
        return score


@click.command()
@click.argument("dataset")
@click.option('-dim', default=8, help="Number of factors.")
@click.option('-iter', default=20, help="Number of epochs.")
@click.option('-learn_rate', default=0.01, help="Learning rate.")
@click.option('-regular', default=0.0001, help="Regularization.")
@click.option('-device', default="cpu", help="Device to use.")
def main(dataset: str, dim: int, iter: int, learn_rate: float, regular: float, device: str):
    train, test = load_dataset(dataset)
    model = FactorizationMachine(train.num_fields, train.num_features, dim)
    model.fit(train, test, iter, learn_rate, regular, device=device)


if __name__ == '__main__':
    main()
