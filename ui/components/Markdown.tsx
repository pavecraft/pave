"use client";

import ReactMarkdown from "react-markdown";

interface Props {
  children: string;
}

export default function Markdown({ children }: Props) {
  return (
    <div className="markdown">
      <ReactMarkdown>{children}</ReactMarkdown>
    </div>
  );
}
