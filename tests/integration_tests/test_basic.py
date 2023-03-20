from pathlib import Path
import pytest
from .network import setup_custom_evmos, setup_evmos

@pytest.fixture(scope="module")
def custom_evmos(tmp_path_factory):
    path = tmp_path_factory.mktemp("filters")
    yield from setup_evmos(path, 26200, long_timeout_commit=True)


@pytest.fixture(scope="module")
def evmos_indexer(tmp_path_factory):
    path = tmp_path_factory.mktemp("indexer")
    yield from setup_custom_evmos(
        path, 26660, Path(__file__).parent / "configs/enable-indexer.jsonnet"
    )


@pytest.fixture(
    scope="module", params=["evmos", "geth", "evmos-ws", "enable-indexer"]
)
def cluster(request, custom_evmos, evmos_indexer, geth):
    """
    run on both evmos and geth
    """
    provider = request.param
    if provider == "evmos":
        yield custom_evmos
    elif provider == "geth":
        yield geth
    elif provider == "evmos-ws":
        evmos_ws = custom_evmos.copy()
        evmos_ws.use_websocket()
        yield evmos_ws
    elif provider == "enable-indexer":
        yield evmos_indexer
    else:
        raise NotImplementedError


def test_basic(cluster):
    w3 = cluster.w3
    assert w3.eth.chain_id == 9000

