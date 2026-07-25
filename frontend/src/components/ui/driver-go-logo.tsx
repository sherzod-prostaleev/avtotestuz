import React from "react";

interface DriverGoLogoProps extends React.SVGProps<SVGSVGElement> {
  className?: string;
  size?: number;
}

export function DriverGoLogo({ className = "h-9 w-9", size = 512, ...props }: DriverGoLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 512 512"
      width={size}
      height={size}
      className={className}
      {...props}
    >
      <defs>
        <linearGradient id="dgl_bgGrad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#1E293B" />
          <stop offset="50%" stopColor="#0F172A" />
          <stop offset="100%" stopColor="#090D16" />
        </linearGradient>

        <radialGradient id="dgl_amberGlow" cx="50%" cy="38%" r="45%">
          <stop offset="0%" stopColor="#F59E0B" stopOpacity="0.95" />
          <stop offset="45%" stopColor="#D97706" stopOpacity="0.45" />
          <stop offset="100%" stopColor="#B45309" stopOpacity="0" />
        </radialGradient>

        <linearGradient id="dgl_wheelGrad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#64748B" />
          <stop offset="50%" stopColor="#334155" />
          <stop offset="100%" stopColor="#1E293B" />
        </linearGradient>

        <linearGradient id="dgl_amberBulb" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FDE047" />
          <stop offset="30%" stopColor="#F59E0B" />
          <stop offset="100%" stopColor="#B45309" />
        </linearGradient>

        <linearGradient id="dgl_goAccent" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#34D399" />
          <stop offset="100%" stopColor="#059669" />
        </linearGradient>

        <filter id="dgl_shadow3d" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="8" stdDeviation="6" floodColor="#000000" floodOpacity="0.5" />
        </filter>
      </defs>

      <rect width="512" height="512" rx="128" fill="url(#dgl_bgGrad)" />
      <rect x="12" y="12" width="488" height="488" rx="116" fill="none" stroke="#F59E0B" strokeWidth="4" strokeOpacity="0.35" />

      <circle cx="256" cy="195" r="140" fill="url(#dgl_amberGlow)" />
      <circle cx="256" cy="225" r="145" fill="none" stroke="url(#dgl_wheelGrad)" strokeWidth="26" filter="url(#dgl_shadow3d)" />
      <circle cx="256" cy="225" r="145" fill="none" stroke="#F59E0B" strokeWidth="3" strokeOpacity="0.6" />

      <path d="M 111 225 L 182 225 M 330 225 L 401 225" stroke="url(#dgl_wheelGrad)" strokeWidth="22" strokeLinecap="round" filter="url(#dgl_shadow3d)" />
      <path d="M 111 225 L 182 225 M 330 225 L 401 225" stroke="#F59E0B" strokeWidth="3" strokeLinecap="round" strokeOpacity="0.4" />

      <circle cx="256" cy="195" r="56" fill="#0F172A" stroke="url(#dgl_amberBulb)" strokeWidth="6" filter="url(#dgl_shadow3d)" />
      <circle cx="256" cy="195" r="36" fill="url(#dgl_amberBulb)" />
      <circle cx="244" cy="183" r="10" fill="#FFFFFF" fillOpacity="0.65" />

      <path d="M 216 250 L 135 385 H 377 L 296 250 Z" fill="#1E293B" stroke="#334155" strokeWidth="4" />
      <path d="M 256 258 L 256 282 M 256 302 L 256 335 M 256 355 L 256 380" stroke="url(#dgl_amberBulb)" strokeWidth="7" strokeLinecap="round" />
      <path d="M 162 318 A 145 145 0 0 0 350 318" fill="none" stroke="url(#dgl_wheelGrad)" strokeWidth="22" strokeLinecap="round" filter="url(#dgl_shadow3d)" />

      <g transform="translate(0, 395)">
        <text x="225" y="44" fontFamily="'Plus Jakarta Sans', 'Inter', sans-serif" fontWeight="900" fontSize="44" fill="#F8FAFC" letterSpacing="1" textAnchor="end">DRIVER</text>
        <text x="240" y="44" fontFamily="'Plus Jakarta Sans', 'Inter', sans-serif" fontWeight="900" fontSize="44" fill="url(#dgl_goAccent)" letterSpacing="1" textAnchor="start">GO</text>
        <path d="M 326 16 L 348 29 L 326 42 L 333 29 Z" fill="url(#dgl_goAccent)" />
      </g>
    </svg>
  );
}
