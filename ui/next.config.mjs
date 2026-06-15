/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  // pg and @libsql native bits must stay external to the server bundle.
  serverExternalPackages: ["pg", "@libsql/client"],
};

export default nextConfig;
