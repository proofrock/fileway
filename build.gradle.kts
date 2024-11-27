plugins {
    java
    application
}

group = "org.example"
version = "0.1.0"

repositories {
    mavenCentral()
}

application {
    mainClass = "it.germanorizzo.proj.fileconduit.Main"
}

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21
}

tasks.withType<JavaCompile> {
    options.encoding = "UTF-8"
    options.release.set(21)
}

dependencies {
    implementation("io.javalin:javalin:6.3.0")
    implementation("com.fasterxml.jackson.core:jackson-databind:2.17.2")
    implementation("org.slf4j:slf4j-simple:2.0.16")
    testImplementation(platform("org.junit:junit-bom:5.10.0"))
    testImplementation("org.junit.jupiter:junit-jupiter")
}

tasks.register("buildDocker") {
    dependsOn("jar")
    dependsOn("copyDependencies")
    doLast {
        exec {
            workingDir(".")
            commandLine("docker", "buildx", "build",
                "-t", "fileconduit:v0.1.0",
                "-f", "Dockerfile",
                "--no-cache",
                "."
            )
        }
    }
}

tasks.test {
    useJUnitPlatform()
}

tasks.register<Copy>("copyDependencies") {
    from(configurations.runtimeClasspath)
    into("${buildDir}/libs")
}

tasks.jar {
    finalizedBy("copyDependencies")
}
