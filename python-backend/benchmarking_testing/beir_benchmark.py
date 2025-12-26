# python-backend/beir_benchmark.py
import argparse
import logging
import os
import pathlib
import sys
from typing import Dict

# Add the grandparent directory (python-backend/) to sys.path
script_dir = os.path.dirname(__file__)  # benchmarking_testing/
parent_dir = os.path.dirname(script_dir)  # python-backend/
sys.path.insert(0, parent_dir)

# Import from your existing app
sys.path.append("app")
# BEIR imports
from beir import util
from beir.datasets.data_loader import GenericDataLoader
from beir.retrieval.evaluation import EvaluateRetrieval

from app.config import get_settings
from app.services.vector_store import VectorStoreService

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class IsolatedChromaRetriever:
    def __init__(self, collection_name: str):
        self.collection_name = collection_name
        self.vector_store = VectorStoreService()
        self.collection = self.vector_store.get_or_create_collection(collection_name)

    def add_corpus(self, corpus: Dict[str, Dict[str, str]]):
        ids = []
        documents = []
        metadatas = []

        for doc_id, doc_data in corpus.items():
            text = doc_data.get("text", "")
            title = doc_data.get("title", "")

            ids.append(doc_id)
            documents.append(text)
            metadatas.append({"title": title, "doc_id": doc_id})

        # Use your embedding model
        embeddings = self.vector_store.embedding_model.encode(documents).tolist()

        self.collection.add(
            ids=ids, documents=documents, metadatas=metadatas, embeddings=embeddings
        )
        logger.info(
            f"Added {len(ids)} docs to isolated collection '{self.collection_name}'"
        )

    def search(
        self,
        corpus: Dict[str, Dict[str, str]],
        queries: Dict[str, str],
        top_k: int = 10,
        score_function=None,
        **kwargs,
    ) -> Dict[str, Dict[str, float]]:
        results = {}

        for qid, query in queries.items():
            search_results = self.vector_store.search(
                query=query, collection_name=self.collection_name, top_k=top_k
            )

            query_results = {res.chunk_id: res.score for res in search_results}
            results[qid] = query_results

        return results


def run_benchmark(dataset: str, collection: str, top_k: int = 10):
    logger.info(f"Starting benchmark for {dataset}")

    # Download dataset
    url = f"https://public.ukp.informatik.tu-darmstadt.de/thakur/BEIR/datasets/{dataset}.zip"
    out_dir = pathlib.Path("datasets")
    out_dir.mkdir(exist_ok=True)
    data_path = util.download_and_unzip(url, out_dir)

    # Load dataset
    corpus, queries, qrels = GenericDataLoader(data_folder=data_path).load(split="test")

    # Initialize retriever
    retriever = IsolatedChromaRetriever(collection_name=collection)
    retriever.add_corpus(corpus)

    # Evaluate
    evaluator = EvaluateRetrieval(retriever, score_function="cos_sim")
    results = evaluator.retrieve(corpus, queries)
    ndcg, _map, recall, precision = evaluator.evaluate(
        qrels, results, evaluator.k_values
    )

    logger.info("Benchmark Results:")
    logger.info(f"NDCG@{top_k}: {ndcg[f'NDCG@{top_k}']}")
    logger.info(f"MAP@{top_k}: {_map[f'MAP@{top_k}']}")
    logger.info(f"Recall@{top_k}: {recall[f'Recall@{top_k}']}")
    logger.info(f"P@{top_k}: {precision[f'P@{top_k}']}")

    return {"ndcg": ndcg, "map": _map, "recall": recall, "precision": precision}


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Run isolated BEIR benchmark")
    parser.add_argument("--dataset", default="scifact", help="BEIR dataset")
    parser.add_argument("--collection", default="beir_test", help="Chroma collection")
    parser.add_argument("--top_k", type=int, default=10, help="Top-k")

    args = parser.parse_args()
    run_benchmark(args.dataset, args.collection, args.top_k)
