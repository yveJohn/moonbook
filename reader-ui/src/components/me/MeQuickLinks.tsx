import { Heart, MessageSquare, Settings } from 'lucide-react';
import { Link } from 'react-router-dom';

const quickLinks = [
  { to: '/me/likes', label: '赞过', Icon: Heart },
  { to: '/me/feedback', label: '意见反馈', Icon: MessageSquare },
  { to: '/me/settings', label: '设置', Icon: Settings }
];

export function MeQuickLinks() {
  return (
    <nav className="me-quick-links" aria-label="我的功能" data-section="quick-links">
      {quickLinks.map(({ to, label, Icon }) => (
        <Link key={to} className="me-quick-link" to={to}>
          <Icon aria-hidden="true" size={22} strokeWidth={2} />
          <span>{label}</span>
        </Link>
      ))}
    </nav>
  );
}
