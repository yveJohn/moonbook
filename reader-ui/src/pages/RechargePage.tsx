import { useEffect, useMemo, useState } from 'react';
import { Check, Gem, LoaderCircle, ShieldCheck } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { createRechargeOrder, getRechargeCatalog } from '../api/reader';
import { useReaderAuth } from '../auth/ReaderAuthContext';
import { AppShell } from '../components/AppShell';
import type { ReaderRechargeCatalog, ReaderRechargeProduct } from '../types/reader';

function requestId() {
  return typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function comparePositiveInteger(left: string, right: string) {
  if (left.length !== right.length) return left.length - right.length;
  return left.localeCompare(right);
}

export function calculateRechargeUsdt(diamondAmount: string, diamondsPerUsdt: string): string | null {
  if (!/^\d+$/.test(diamondAmount)) return null;
  const rateMatch = /^(\d+)(?:\.(\d+))?$/.exec(diamondsPerUsdt.trim());
  if (!rateMatch) return null;
  const fraction = rateMatch[2] || '';
  const denominator = 10n ** BigInt(fraction.length);
  const rateNumerator = BigInt(rateMatch[1]) * denominator + BigInt(fraction || '0');
  if (rateNumerator <= 0n) return null;
  const centsNumerator = BigInt(diamondAmount) * 100n * denominator;
  const cents = (centsNumerator + rateNumerator - 1n) / rateNumerator;
  return `${cents / 100n}.${(cents % 100n).toString().padStart(2, '0')}`;
}

export function RechargePage() {
  const navigate = useNavigate();
  const { isAuthenticated, sessionReady } = useReaderAuth();
  const [catalog, setCatalog] = useState<ReaderRechargeCatalog | null>(null);
  const [custom, setCustom] = useState('');
  const [error, setError] = useState('');
  const [creating, setCreating] = useState('');
  const [selectedProductId, setSelectedProductId] = useState('');
  const limits = useMemo(() => catalog ? { min: catalog.minDiamondAmount, max: catalog.maxDiamondAmount } : null, [catalog]);
  const customInRange = Boolean(limits && /^\d+$/.test(custom)
    && comparePositiveInteger(custom, limits.min) >= 0
    && comparePositiveInteger(custom, limits.max) <= 0);
  const customPriceUsdt = useMemo(() => customInRange && catalog
    ? calculateRechargeUsdt(custom, catalog.diamondsPerUsdt)
    : null, [catalog, custom, customInRange]);
  const customRangeError = custom && limits && !customInRange
    ? `钻石数量必须在 ${limits.min} 到 ${limits.max} 之间`
    : '';

  useEffect(() => {
    if (!sessionReady) return;
    if (!isAuthenticated) {
      navigate('/auth/login?redirect=/me/recharge', { replace: true });
      return;
    }
    getRechargeCatalog().then((value) => {
      setCatalog(value);
      setSelectedProductId(value.products[0]?.id || '');
    }).catch((value) => setError(value instanceof Error ? value.message : '充值档位加载失败'));
  }, [isAuthenticated, navigate, sessionReady]);

  const create = async (product?: ReaderRechargeProduct) => {
    setError('');
    const key = product?.id || 'custom';
    setCreating(key);
    try {
      const order = await createRechargeOrder(product
        ? { productId: product.id, requestId: requestId() }
        : { customDiamondAmount: custom, requestId: requestId() });
      navigate(`/me/recharge/orders/${order.id}`);
    } catch (value) {
      setError(value instanceof Error ? value.message : '创建充值订单失败');
    } finally {
      setCreating('');
    }
  };

  const updateCustom = (value: string) => {
    const cleaned = value.replace(/\D/g, '').replace(/^0+/, '');
    setCustom(cleaned);
    if (cleaned) setSelectedProductId('');
    setError('');
  };

  const selectProduct = (productId: string) => {
    setSelectedProductId(productId);
    setCustom('');
    setError('');
  };

  const selectedProduct = catalog?.products.find((product) => product.id === selectedProductId);
  const displayedPrice = customPriceUsdt || selectedProduct?.priceUsdt || '';
  const canSubmit = Boolean(selectedProduct || customPriceUsdt) && !creating;

  const submit = () => {
    if (selectedProduct) {
      void create(selectedProduct);
    } else if (customPriceUsdt) {
      void create();
    }
  };

  return (
    <AppShell title="充值中心" back onBack={() => navigate('/me/diamonds')} hideTabs>
      <main className="recharge-page">
        <section className="recharge-balance-band">
          <span className="recharge-balance-band__icon"><Gem aria-hidden="true" /></span>
          <div className="recharge-balance-band__copy"><strong>TRC20-USDT</strong><span>到账后自动兑换钻石</span></div>
          <span className="recharge-balance-band__safe"><ShieldCheck aria-hidden="true" />安全可靠</span>
        </section>

        <section className="recharge-section" aria-labelledby="recharge-products-title">
          <div className="recharge-section__heading">
            <h2 id="recharge-products-title">选择充值档位</h2>
            <span>充值后钻石将自动到账</span>
          </div>
          <div className="recharge-product-grid">
            {catalog?.products.map((product) => (
              <button
                key={product.id}
                type="button"
                className={selectedProductId === product.id ? 'recharge-product is-selected' : 'recharge-product'}
                disabled={Boolean(creating)}
                aria-pressed={selectedProductId === product.id}
                onClick={() => selectProduct(product.id)}
              >
                <Gem className="recharge-product__watermark" aria-hidden="true" />
                <strong><b>{product.diamondAmount}</b> 钻石</strong>
                <span>{product.priceUsdt} USDT</span>
                {selectedProductId === product.id ? <i className="recharge-product__corner"><Check aria-hidden="true" /></i> : null}
                {creating === product.id ? <LoaderCircle className="spin" aria-label="创建中" /> : null}
              </button>
            ))}
          </div>
        </section>

        <aside className="recharge-network-notice">
          <ShieldCheck aria-hidden="true" />
          <span>仅支持 TRC20 网络的 USDT 转账，请勿使用其他链转账</span>
        </aside>

        {catalog?.customEnabled ? (
          <section className="recharge-section" aria-labelledby="custom-recharge-title">
            <div className="recharge-section__heading recharge-section__heading--stacked">
              <h2 id="custom-recharge-title">自定义钻石数量</h2>
              <span>输入想要充值的钻石数量，最低 {catalog.minDiamondAmount}，最高 {catalog.maxDiamondAmount}</span>
            </div>
            <div className="recharge-custom-row">
              <label><span><Gem aria-hidden="true" />钻石</span><input aria-label="自定义钻石数量" value={custom} inputMode="numeric" placeholder={`${catalog.minDiamondAmount}-${catalog.maxDiamondAmount}`} onChange={(event) => updateCustom(event.target.value)} /></label>
            </div>
            <div className="recharge-quote" aria-live="polite"><span>预计支付</span><strong>{displayedPrice ? `${displayedPrice} USDT` : '-- USDT'}</strong></div>
            {customRangeError ? <p className="recharge-error" role="alert">{customRangeError}</p> : null}
          </section>
        ) : null}
        {error ? <p className="recharge-error" role="alert">{error}</p> : null}

        <div className="recharge-trust"><span /><p><ShieldCheck aria-hidden="true" />资金安全有保障，到账后自动兑换钻石</p><span /></div>
        <button type="button" className="recharge-submit" disabled={!canSubmit} onClick={submit}>
          {creating ? <><LoaderCircle className="spin" aria-hidden="true" />创建中</> : '立即充值'}
        </button>
      </main>
    </AppShell>
  );
}
